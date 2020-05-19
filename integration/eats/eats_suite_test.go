package eats_test

import (
	"bytes"
	"crypto/tls"
	"crypto/x509"
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
	"os/exec"
	"testing"
	"time"

	"code.cloudfoundry.org/cfhttp/v2"
	"code.cloudfoundry.org/eirini"
	"code.cloudfoundry.org/eirini/integration/util"
	"code.cloudfoundry.org/eirini/models/cf"
	. "github.com/onsi/ginkgo"
	ginkgoconfig "github.com/onsi/ginkgo/config"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gexec"
	"k8s.io/client-go/rest"
)

func TestEats(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Eats Suite")
}

type BinPaths struct {
	OPI                      string `json:"opi"`
	RouteCollector           string `json:"route_collector"`
	MetricsCollector         string `json:"metrics_collector"`
	RouteStatefulsetInformer string `json:"route_stateful_set_informer"`
	RoutePodInformer         string `json:"route_pod_informer"`
	EventsReporter           string `json:"events_reporter"`
	TaskReporter             string `json:"task_reporter"`
}

type routeInfo struct {
	Hostname string `json:"hostname"`
	Port     int    `json:"port"`
}

var (
	fixture *util.Fixture

	localhostCertPath, localhostKeyPath string
	opiConfig                           string
	opiSession                          *gexec.Session
	httpClient                          *http.Client
	opiURL                              string
	binPaths                            BinPaths
)

var _ = SynchronizedBeforeSuite(func() []byte {
	paths := BinPaths{}

	var err error
	paths.OPI, err = gexec.Build("code.cloudfoundry.org/eirini/cmd/opi")
	Expect(err).NotTo(HaveOccurred())

	paths.RouteCollector, err = gexec.Build("code.cloudfoundry.org/eirini/cmd/route-collector")
	Expect(err).NotTo(HaveOccurred())

	paths.MetricsCollector, err = gexec.Build("code.cloudfoundry.org/eirini/cmd/metrics-collector")
	Expect(err).NotTo(HaveOccurred())

	paths.RouteStatefulsetInformer, err = gexec.Build("code.cloudfoundry.org/eirini/cmd/route-statefulset-informer")
	Expect(err).NotTo(HaveOccurred())

	paths.RoutePodInformer, err = gexec.Build("code.cloudfoundry.org/eirini/cmd/route-pod-informer")
	Expect(err).NotTo(HaveOccurred())

	paths.EventsReporter, err = gexec.Build("code.cloudfoundry.org/eirini/cmd/event-reporter")
	Expect(err).NotTo(HaveOccurred())

	paths.TaskReporter, err = gexec.Build("code.cloudfoundry.org/eirini/cmd/task-reporter")
	Expect(err).NotTo(HaveOccurred())

	data, err := json.Marshal(paths)
	Expect(err).NotTo(HaveOccurred())

	return data
}, func(data []byte) {
	err := json.Unmarshal(data, &binPaths)
	Expect(err).NotTo(HaveOccurred())

	fixture = util.NewFixture(GinkgoWriter)
	SetDefaultEventuallyTimeout(20 * time.Second)
})

var _ = BeforeEach(func() {
	fixture.SetUp()

	localhostCertPath, localhostKeyPath = util.GenerateKeyPair("localhost")

	var err error
	httpClient, err = makeTestHTTPClient(localhostCertPath, localhostKeyPath)
	Expect(err).ToNot(HaveOccurred())

	opiSession, opiConfig, opiURL = runOpi(localhostCertPath, localhostKeyPath)
	waitOpiReady(httpClient, opiURL)
})

var _ = AfterEach(func() {
	fixture.TearDown()

	if opiSession != nil {
		opiSession.Kill()
	}
	Expect(os.Remove(opiConfig)).To(Succeed())
	Expect(os.Remove(localhostCertPath)).To(Succeed())
	Expect(os.Remove(localhostKeyPath)).To(Succeed())
})

func runOpi(certPath, keyPath string) (*gexec.Session, string, string) {
	eiriniConfig := &eirini.Config{
		Properties: eirini.Properties{
			KubeConfig: eirini.KubeConfig{
				ConfigPath: fixture.KubeConfigPath,
				Namespace:  fixture.Namespace,
			},
			CCCAPath:             certPath,
			CCCertPath:           certPath,
			CCKeyPath:            keyPath,
			ServerCertPath:       certPath,
			ServerKeyPath:        keyPath,
			ClientCAPath:         certPath,
			DiskLimitMB:          500,
			TLSPort:              61000 + ginkgoconfig.GinkgoConfig.ParallelNode,
			CCUploaderSecretName: "cc-uploader-secret",
			CCUploaderCertPath:   "path-to-crt",
			CCUploaderKeyPath:    "path-to-key",

			ClientCertsSecretName: "eirini-client-secret",
			ClientKeyPath:         "path-to-key",
			ClientCertPath:        "path-to-crt",

			CACertSecretName: "global-ca-secret",
			CACertPath:       "path-to-ca",

			DownloaderImage: "docker.io/eirini/integration_test_staging",
			ExecutorImage:   "docker.io/eirini/integration_test_staging",
			UploaderImage:   "docker.io/eirini/integration_test_staging",

			ApplicationServiceAccount: "default",
		},
	}
	eiriniConfigFile, err := util.CreateConfigFile(eiriniConfig)
	Expect(err).ToNot(HaveOccurred())

	eiriniCommand := exec.Command(binPaths.OPI, "connect", "-c", eiriniConfigFile.Name()) // #nosec G204
	eiriniSession, err := gexec.Start(eiriniCommand, GinkgoWriter, GinkgoWriter)
	Expect(err).ToNot(HaveOccurred())

	url := fmt.Sprintf("https://localhost:%d", eiriniConfig.Properties.TLSPort)

	return eiriniSession, eiriniConfigFile.Name(), url
}

func makeTestHTTPClient(certPath, keyPath string) (*http.Client, error) {
	bs, err := ioutil.ReadFile(certPath)
	if err != nil {
		return nil, err
	}

	clientCert, err := tls.LoadX509KeyPair(certPath, keyPath)
	if err != nil {
		return nil, err
	}

	certPool := x509.NewCertPool()
	if !certPool.AppendCertsFromPEM(bs) {
		return nil, err
	}
	tlsConfig := &tls.Config{
		RootCAs:      certPool,
		Certificates: []tls.Certificate{clientCert},
	}
	client := cfhttp.NewClient(cfhttp.WithTLSConfig(tlsConfig))

	return client, nil
}

func waitOpiReady(httpClient rest.HTTPClient, opiURL string) {
	Eventually(func() error {
		desireAppReq, err := http.NewRequest("GET", fmt.Sprintf("%s/apps", opiURL), bytes.NewReader([]byte{}))
		Expect(err).ToNot(HaveOccurred())
		_, err = httpClient.Do(desireAppReq) //nolint:bodyclose
		return err
	}).Should(Succeed())
}

func getLRP(processGUID, versionGUID string) (cf.DesiredLRP, error) {
	request, err := http.NewRequest("GET", fmt.Sprintf("%s/apps/%s/%s", opiURL, processGUID, versionGUID), nil)
	if err != nil {
		return cf.DesiredLRP{}, err
	}

	response, err := httpClient.Do(request)
	if err != nil {
		return cf.DesiredLRP{}, err
	}
	defer response.Body.Close()

	if response.StatusCode >= 400 {
		return cf.DesiredLRP{}, errors.New(response.Status)
	}

	var desiredLRPResponse cf.DesiredLRPResponse
	if err := json.NewDecoder(response.Body).Decode(&desiredLRPResponse); err != nil {
		return cf.DesiredLRP{}, err
	}

	return desiredLRPResponse.DesiredLRP, nil
}

func getLRPs() ([]cf.DesiredLRPSchedulingInfo, error) {
	request, err := http.NewRequest("GET", fmt.Sprintf("%s/apps", opiURL), nil)
	if err != nil {
		return nil, err
	}

	response, err := httpClient.Do(request)
	if err != nil {
		return nil, err
	}
	defer response.Body.Close()

	if response.StatusCode >= 400 {
		return nil, errors.New(response.Status)
	}

	var desiredLRPSchedulingInfoResponse cf.DesiredLRPSchedulingInfosResponse
	decoder := json.NewDecoder(response.Body)
	err = decoder.Decode(&desiredLRPSchedulingInfoResponse)
	Expect(err).ToNot(HaveOccurred())
	return desiredLRPSchedulingInfoResponse.DesiredLrpSchedulingInfos, nil
}

func getInstances(processGUID, versionGUID string) (*cf.GetInstancesResponse, error) {
	request, err := http.NewRequest("GET", fmt.Sprintf("%s/apps/%s/%s/instances", opiURL, processGUID, versionGUID), nil)
	if err != nil {
		return nil, err
	}

	response, err := httpClient.Do(request)
	if err != nil {
		return nil, err
	}
	defer response.Body.Close()

	var instancesResponse *cf.GetInstancesResponse
	err = json.NewDecoder(response.Body).Decode(&instancesResponse)
	if err != nil {
		return nil, err
	}

	if response.StatusCode >= 400 {
		return instancesResponse, errors.New(response.Status)
	}

	return instancesResponse, nil
}

func desireLRP(lrpRequest cf.DesireLRPRequest) *http.Response {
	body, err := json.Marshal(lrpRequest)
	Expect(err).NotTo(HaveOccurred())
	desireLrpReq, err := http.NewRequest("PUT", fmt.Sprintf("%s/apps/%s", opiURL, lrpRequest.GUID), bytes.NewReader(body))
	Expect(err).NotTo(HaveOccurred())
	response, err := httpClient.Do(desireLrpReq)
	Expect(err).NotTo(HaveOccurred())
	return response
}

func stopLRP(httpClient rest.HTTPClient, opiURL, processGUID, versionGUID string) (*http.Response, error) {
	request, err := http.NewRequest("PUT", fmt.Sprintf("%s/apps/%s/%s/stop", opiURL, processGUID, versionGUID), nil)
	if err != nil {
		return nil, err
	}
	return httpClient.Do(request)
}

func stopLRPInstance(processGUID, versionGUID string, instance int) (*http.Response, error) {
	request, err := http.NewRequest("PUT", fmt.Sprintf("%s/apps/%s/%s/stop/%d", opiURL, processGUID, versionGUID, instance), nil)
	if err != nil {
		return nil, err
	}
	return httpClient.Do(request)
}

func updateLRP(updateRequest cf.UpdateDesiredLRPRequest) (*http.Response, error) {
	body, err := json.Marshal(updateRequest)
	if err != nil {
		return nil, err
	}
	updateLrpReq, err := http.NewRequest("POST", fmt.Sprintf("%s/apps/%s", opiURL, updateRequest.GUID), bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	return httpClient.Do(updateLrpReq)
}

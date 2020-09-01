package eats_test

import (
	"bytes"
	"context"
	"crypto/tls"
	"crypto/x509"
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
	"testing"
	"time"

	"code.cloudfoundry.org/cfhttp/v2"
	"code.cloudfoundry.org/eirini/k8s"
	"code.cloudfoundry.org/eirini/models/cf"
	"code.cloudfoundry.org/eirini/tests"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	batchv1 "k8s.io/api/batch/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/rest"
)

func TestEats(t *testing.T) {
	SetDefaultEventuallyTimeout(4 * time.Minute)
	RegisterFailHandler(Fail)
	RunSpecs(t, "Eats Suite")
}

type routeInfo struct {
	Hostname string `json:"hostname"`
	Port     int    `json:"port"`
}

var (
	fixture    *tests.Fixture
	binsPath   string
	eiriniBins tests.EiriniBinaries

	eiriniCertPath, eiriniKeyPath string
	httpClient                    *http.Client
	opiURL                        string
)

var _ = SynchronizedBeforeSuite(func() []byte {
	var err error
	binsPath, err = ioutil.TempDir("", "bins")
	Expect(err).NotTo(HaveOccurred())

	eiriniBins = tests.NewEiriniBinaries(binsPath)

	data, err := json.Marshal(eiriniBins)
	Expect(err).NotTo(HaveOccurred())

	return data
}, func(data []byte) {
	err := json.Unmarshal(data, &eiriniBins)
	Expect(err).NotTo(HaveOccurred())

	fixture = tests.NewFixture(GinkgoWriter)
})

var _ = SynchronizedAfterSuite(func() {
	fixture.Destroy()
}, func() {
	eiriniBins.TearDown()
	Expect(os.RemoveAll(binsPath)).To(Succeed())
})

var _ = BeforeEach(func() {
	fixture.SetUp()

	var err error
	eiriniCertPath, eiriniKeyPath = fixture.DownloadEiriniCertificates()

	httpClient, err = makeTestHTTPClient(eiriniCertPath, eiriniKeyPath)
	Expect(err).ToNot(HaveOccurred())

	svc, getErr := fixture.Clientset.CoreV1().Services("eirini-core").Get(context.Background(), "eirini-external", metav1.GetOptions{})
	Expect(getErr).ToNot(HaveOccurred())

	host := svc.Status.LoadBalancer.Ingress[0].IP
	opiURL = fmt.Sprintf("https://%s:8085", host)
})

var _ = AfterEach(func() {
	fixture.TearDown()

	Expect(os.Remove(eiriniCertPath)).To(Succeed())
	Expect(os.Remove(eiriniKeyPath)).To(Succeed())
})

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

func getPodReadiness(lrpGUID, lrpVersion string) bool {
	pods, err := fixture.Clientset.CoreV1().Pods(fixture.Namespace).List(context.Background(), metav1.ListOptions{
		LabelSelector: fmt.Sprintf("%s=%s,%s=%s", k8s.LabelGUID, lrpGUID, k8s.LabelVersion, lrpVersion),
	})
	Expect(err).NotTo(HaveOccurred())

	if len(pods.Items) != 1 {
		return false
	}

	containerStatuses := pods.Items[0].Status.ContainerStatuses
	if len(containerStatuses) != 1 {
		return false
	}

	return containerStatuses[0].Ready
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

func listJobs() []batchv1.Job {
	jobs, err := fixture.Clientset.
		BatchV1().
		Jobs(fixture.Namespace).
		List(context.Background(), metav1.ListOptions{})

	Expect(err).NotTo(HaveOccurred())

	return jobs.Items
}

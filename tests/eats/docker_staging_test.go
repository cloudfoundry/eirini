package eats_test

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"os"

	"code.cloudfoundry.org/eirini"
	"code.cloudfoundry.org/eirini/bifrost"
	"code.cloudfoundry.org/eirini/models/cf"
	"code.cloudfoundry.org/eirini/tests"
	"code.cloudfoundry.org/runtimeschema/cc_messages"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gexec"
	"github.com/onsi/gomega/ghttp"
	"k8s.io/client-go/rest"
)

var _ = Describe("Docker Staging", func() {
	var (
		httpClient rest.HTTPClient
		capiServer *ghttp.Server
		opiConfig  string
		opiSession *gexec.Session
		certPath   string
		keyPath    string
	)

	BeforeEach(func() {
		certPath, keyPath = tests.GenerateKeyPair("localhost")

		opiSession, opiConfig, opiURL = runOpi(certPath, keyPath)

		var err error
		httpClient, err = makeTestHTTPClient(certPath, keyPath)
		Expect(err).ToNot(HaveOccurred())

		waitOpiReady(httpClient, opiURL)

		capiServer, err = tests.CreateTestServer(
			certPath, keyPath, certPath,
		)
		Expect(err).NotTo(HaveOccurred())
		capiServer.HTTPTestServer.StartTLS()

		capiServer.RouteToHandler(
			"POST",
			"/staging/completed",
			func(w http.ResponseWriter, req *http.Request) {
				bytes, err := ioutil.ReadAll(req.Body)
				Expect(err).NotTo(HaveOccurred())
				request := &cc_messages.StagingResponseForCC{}
				Expect(json.Unmarshal(bytes, request)).To(Succeed())
				Expect(request.Error).To(BeNil())
				stagingResult := bifrost.StagingResult{}
				Expect(json.Unmarshal(*request.Result, &stagingResult)).To(Succeed())
				Expect(stagingResult.ExecutionMetadata).To(Equal(`{"cmd":[],"ports":[{"Port":8888,"Protocol":"tcp"}]}`))
				Expect(stagingResult.LifecycleType).To(Equal("docker"))
				Expect(stagingResult.LifecycleMetadata.DockerImage).To(Equal("eirini/custom-port"))
				Expect(stagingResult.ProcessTypes).To(Equal(bifrost.ProcessTypes{Web: ""}))
			},
		)
	})

	AfterEach(func() {
		if opiSession != nil {
			opiSession.Kill()
		}
		Expect(os.Remove(opiConfig)).To(Succeed())

		capiServer.Close()
	})

	It("returns code 201 Accepted and completes staging", func() {
		code, err := desireStaging(httpClient, cf.StagingRequest{
			Lifecycle: cf.StagingLifecycle{
				DockerLifecycle: &cf.StagingDockerLifecycle{
					Image: "eirini/custom-port",
				},
			},
			CompletionCallback: fmt.Sprintf("%s/staging/completed", capiServer.URL()),
		})
		Expect(err).NotTo(HaveOccurred())
		Expect(code).To(Equal(http.StatusAccepted))

		Expect(capiServer.ReceivedRequests()).To(HaveLen(1))
	})

	When("image lives in a private registry", func() {
		BeforeEach(func() {
			capiServer.RouteToHandler(
				"POST",
				"/staging/completed",
				func(w http.ResponseWriter, req *http.Request) {
					bytes, err := ioutil.ReadAll(req.Body)
					Expect(err).NotTo(HaveOccurred())
					request := &cc_messages.StagingResponseForCC{}
					Expect(json.Unmarshal(bytes, request)).To(Succeed())
					Expect(request.Error).To(BeNil())
					stagingResult := bifrost.StagingResult{}
					Expect(json.Unmarshal(*request.Result, &stagingResult)).To(Succeed())
					Expect(stagingResult.ExecutionMetadata).To(Equal(`{"cmd":[],"ports":[{"Port":8888,"Protocol":"tcp"}]}`))
					Expect(stagingResult.LifecycleMetadata.DockerImage).To(Equal("eiriniuser/notdora"))
				},
			)
		})

		It("returns code 201 Accepted and completes staging", func() {
			code, err := desireStaging(httpClient, cf.StagingRequest{
				Lifecycle: cf.StagingLifecycle{
					DockerLifecycle: &cf.StagingDockerLifecycle{
						Image:            "eiriniuser/notdora",
						RegistryUsername: "eiriniuser",
						RegistryPassword: tests.GetEiriniDockerHubPassword(),
					},
				},
				CompletionCallback: fmt.Sprintf("%s/staging/completed", capiServer.URL()),
			})
			Expect(err).NotTo(HaveOccurred())
			Expect(code).To(Equal(http.StatusAccepted))

			Expect(capiServer.ReceivedRequests()).To(HaveLen(1))
		})
	})

	When("the callback uri is invalid", func() {
		It("should return a 500 Internal Server Error", func() {
			code, err := desireStaging(httpClient, cf.StagingRequest{
				Lifecycle: cf.StagingLifecycle{
					DockerLifecycle: &cf.StagingDockerLifecycle{
						Image: "eirini/custom-port",
					},
				},
				CompletionCallback: "http://definitely-does-not-exist.io/staging/completed",
			})
			Expect(err).NotTo(HaveOccurred())
			Expect(code).To(Equal(http.StatusInternalServerError))
		})
	})

	When("the image is invalid", func() {
		BeforeEach(func() {
			capiServer.RouteToHandler(
				"POST",
				"/staging/completed",
				func(w http.ResponseWriter, req *http.Request) {
					bytes, err := ioutil.ReadAll(req.Body)
					Expect(err).NotTo(HaveOccurred())
					request := &cc_messages.StagingResponseForCC{}
					Expect(json.Unmarshal(bytes, request)).To(Succeed())
					Expect(request.Error).NotTo(BeNil())
					Expect(request.Error.Id).To(Equal(cc_messages.STAGING_ERROR))
					Expect(request.Error.Message).To(ContainSubstring("failed to parse image ref"))
				},
			)
		})

		It("should return a 500 Internal Server Error", func() {
			code, err := desireStaging(httpClient, cf.StagingRequest{
				Lifecycle: cf.StagingLifecycle{
					DockerLifecycle: &cf.StagingDockerLifecycle{
						Image: "what is eirini",
					},
				},
				CompletionCallback: fmt.Sprintf("%s/staging/completed", capiServer.URL()),
			})
			Expect(err).NotTo(HaveOccurred())
			Expect(code).To(Equal(http.StatusAccepted))
		})
	})
})

func runOpi(certPath, keyPath string) (*gexec.Session, string, string) {
	eiriniConfig := &eirini.Config{
		Properties: eirini.Properties{
			KubeConfig: eirini.KubeConfig{
				ConfigPath: fixture.KubeConfigPath,
				Namespace:  fixture.GetEiriniWorkloadsNamespace(),
			},
			CCCAPath:        certPath,
			CCCertPath:      certPath,
			CCKeyPath:       keyPath,
			ServerCertPath:  certPath,
			ServerKeyPath:   keyPath,
			ClientCAPath:    certPath,
			DiskLimitMB:     500,
			TLSPort:         fixture.NextAvailablePort(),
			DownloaderImage: "docker.io/eirini/integration_test_staging",
			ExecutorImage:   "docker.io/eirini/integration_test_staging",
			UploaderImage:   "docker.io/eirini/integration_test_staging",

			ApplicationServiceAccount: tests.GetApplicationServiceAccount(),
		},
	}

	eiriniSession, eiriniConfigFilePath := eiriniBins.OPI.Run(eiriniConfig)

	url := fmt.Sprintf("https://localhost:%d", eiriniConfig.Properties.TLSPort)

	return eiriniSession, eiriniConfigFilePath, url
}

func desireStaging(httpClient rest.HTTPClient, stagingRequest cf.StagingRequest) (int, error) {
	data, err := json.Marshal(stagingRequest)
	if err != nil {
		return 0, err
	}

	request, err := http.NewRequest("POST", fmt.Sprintf("%s/stage/some-guid", opiURL), bytes.NewReader(data))
	if err != nil {
		return 0, err
	}

	response, err := httpClient.Do(request)
	if err != nil {
		return 0, err
	}

	defer response.Body.Close()

	return response.StatusCode, nil
}

func waitOpiReady(httpClient rest.HTTPClient, opiURL string) {
	Eventually(func() error {
		desireAppReq, err := http.NewRequest("GET", fmt.Sprintf("%s/apps", opiURL), bytes.NewReader([]byte{}))
		Expect(err).ToNot(HaveOccurred())
		_, err = httpClient.Do(desireAppReq) //nolint:bodyclose

		return err
	}).Should(Succeed())
}

package opi_test

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"

	"code.cloudfoundry.org/eirini/bifrost"
	"code.cloudfoundry.org/eirini/models/cf"
	"code.cloudfoundry.org/eirini/tests"
	"code.cloudfoundry.org/runtimeschema/cc_messages"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/ghttp"
	"k8s.io/client-go/rest"
)

var _ = Describe("Docker Staging", func() {
	var capiServer *ghttp.Server

	BeforeEach(func() {
		var err error
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

func desireStaging(httpClient rest.HTTPClient, stagingRequest cf.StagingRequest) (int, error) {
	data, err := json.Marshal(stagingRequest)
	if err != nil {
		return 0, err
	}

	request, err := http.NewRequest("POST", fmt.Sprintf("%s/stage/some-guid", url), bytes.NewReader(data))
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

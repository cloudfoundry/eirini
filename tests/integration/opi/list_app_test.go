package opi_test

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"

	"code.cloudfoundry.org/eirini"
	"code.cloudfoundry.org/eirini/models/cf"
	"code.cloudfoundry.org/eirini/tests"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"k8s.io/client-go/rest"
)

var _ = Describe("ListAppTest", func() {

	var (
		configuredNamespaceAppGUID string
		extraNamespaceAppGUID      string
	)

	BeforeEach(func() {
		configuredNamespaceAppGUID = tests.GenerateGUID()
		extraNamespaceAppGUID = tests.GenerateGUID()
	})

	JustBeforeEach(func() {
		desireLRPWithGUID(configuredNamespaceAppGUID, fixture.Namespace)
		desireLRPWithGUID(extraNamespaceAppGUID, fixture.CreateExtraNamespace())
	})

	It("will list apps in all namespaces", func() {
		apps := listLRPs(httpClient, url)

		guids := []string{}
		for _, lrp := range apps {
			guids = append(guids, lrp.GUID)
		}

		Expect(guids).To(ContainElements(configuredNamespaceAppGUID, extraNamespaceAppGUID))
	})

	When("multi namespace support is disabled", func() {
		var restartedConfigPath string

		AfterEach(func() {
			Expect(os.RemoveAll(restartedConfigPath)).To(Succeed())
		})

		JustBeforeEach(func() {
			restartedConfigPath = restartWithConfig(func(cfg eirini.Config) eirini.Config {
				cfg.Properties.EnableMultiNamespaceSupport = false

				return cfg
			})
		})

		It("will list apps only in the configured namespace", func() {
			apps := listLRPs(httpClient, url)

			Expect(apps).To(HaveLen(1))
			Expect(apps[0].GUID).To(Equal(configuredNamespaceAppGUID))
		})
	})
})

func desireLRPWithGUID(guid, namespace string) {
	request := cf.DesireLRPRequest{
		GUID:      guid,
		Version:   "the-version",
		Namespace: namespace,
		Ports:     []int32{8080},
		DiskMB:    512,
		Lifecycle: cf.Lifecycle{
			DockerLifecycle: &cf.DockerLifecycle{
				Image:   "busybox",
				Command: []string{"/bin/sleep", "100"},
			},
		},
		NumInstances: 1,
	}

	response := desireLRP(httpClient, url, request)
	defer response.Body.Close()
	Expect(response.StatusCode).To(Equal(http.StatusAccepted))
}

func listLRPs(httpClient rest.HTTPClient, url string) []cf.DesiredLRPSchedulingInfo {
	listRequest, err := http.NewRequest("GET", fmt.Sprintf("%s/apps", url), nil)
	Expect(err).NotTo(HaveOccurred())

	response, err := httpClient.Do(listRequest)
	Expect(err).NotTo(HaveOccurred())

	defer response.Body.Close()
	Expect(response.StatusCode).To(Equal(http.StatusOK))

	var desiredLRPs cf.DesiredLRPSchedulingInfosResponse

	decoder := json.NewDecoder(response.Body)
	err = decoder.Decode(&desiredLRPs)
	Expect(err).ToNot(HaveOccurred())

	return desiredLRPs.DesiredLrpSchedulingInfos
}

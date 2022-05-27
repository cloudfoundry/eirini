package api_test

import (
	"encoding/json"
	"fmt"
	"net/http"

	"code.cloudfoundry.org/eirini/models/cf"
	"code.cloudfoundry.org/eirini/tests"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"k8s.io/client-go/rest"
)

var _ = Describe("ListAppTest", func() {
	var configuredNamespaceAppGUID string

	BeforeEach(func() {
		configuredNamespaceAppGUID = tests.GenerateGUID()
	})

	JustBeforeEach(func() {
		desireLRPWithGUID(configuredNamespaceAppGUID, fixture.Namespace)
	})

	It("will list apps", func() {
		apps := listLRPs(httpClient, eiriniAPIUrl)

		Expect(apps).To(HaveLen(1))
		Expect(apps[0].GUID).To(Equal(configuredNamespaceAppGUID))
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
				Image:   "eirini/busybox",
				Command: []string{"/bin/sleep", "100"},
			},
		},
		NumInstances: 1,
	}

	response := desireLRP(httpClient, eiriniAPIUrl, request)
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

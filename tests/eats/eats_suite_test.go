package eats_test

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"testing"
	"time"

	"code.cloudfoundry.org/eirini/k8s"
	"code.cloudfoundry.org/eirini/models/cf"
	"code.cloudfoundry.org/eirini/tests"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/dynamic"
	_ "k8s.io/client-go/plugin/pkg/client/auth"
	"k8s.io/client-go/tools/clientcmd"
)

func TestEats(t *testing.T) {
	SetDefaultEventuallyTimeout(4 * time.Minute)
	RegisterFailHandler(tests.EatsFailHandler)
	RunSpecs(t, "Eats Suite")
}

var fixture *tests.EATSFixture

var _ = SynchronizedBeforeSuite(
	func() []byte {
		Expect(tests.NewWiremock().Reset()).To(Succeed())

		return nil
	},

	func(_ []byte) {
		baseFixture := tests.NewFixture(GinkgoWriter)
		config, err := clientcmd.BuildConfigFromFlags("", baseFixture.KubeConfigPath)
		Expect(err).NotTo(HaveOccurred(), "failed to build config from flags")

		dynamicClientset, err := dynamic.NewForConfig(config)
		Expect(err).NotTo(HaveOccurred(), "failed to create clientset")

		wiremockClient := tests.NewWiremock()

		fixture = tests.NewEATSFixture(*baseFixture, dynamicClientset, wiremockClient)
	},
)

var _ = AfterSuite(func() {
	fixture.Destroy()
})

var _ = BeforeEach(func() {
	fixture.SetUp()
})

var _ = AfterEach(func() {
	fixture.TearDown()
})

func getLRP(processGUID, versionGUID string) (cf.DesiredLRP, error) {
	request, err := http.NewRequest("GET", fmt.Sprintf("%s/apps/%s/%s", tests.GetEiriniAddress(), processGUID, versionGUID), nil)
	if err != nil {
		return cf.DesiredLRP{}, err
	}

	response, err := fixture.GetEiriniHTTPClient().Do(request)
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
	request, err := http.NewRequest("GET", fmt.Sprintf("%s/apps", tests.GetEiriniAddress()), nil)
	if err != nil {
		return nil, err
	}

	response, err := fixture.GetEiriniHTTPClient().Do(request)
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
	if len(containerStatuses) == 0 {
		return false
	}

	for _, cs := range containerStatuses {
		if cs.Ready == false {
			return false
		}
	}

	return true
}

func getInstances(processGUID, versionGUID string) (*cf.GetInstancesResponse, error) {
	request, err := http.NewRequest("GET", fmt.Sprintf("%s/apps/%s/%s/instances", tests.GetEiriniAddress(), processGUID, versionGUID), nil)
	if err != nil {
		return nil, err
	}

	response, err := fixture.GetEiriniHTTPClient().Do(request)
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

func desireLRP(lrpRequest cf.DesireLRPRequest) int {
	body, err := json.Marshal(lrpRequest)
	Expect(err).NotTo(HaveOccurred())
	desireLrpReq, err := http.NewRequest("PUT", fmt.Sprintf("%s/apps/%s", tests.GetEiriniAddress(), lrpRequest.GUID), bytes.NewReader(body))
	Expect(err).NotTo(HaveOccurred())
	response, err := fixture.GetEiriniHTTPClient().Do(desireLrpReq)
	Expect(err).NotTo(HaveOccurred())

	defer response.Body.Close()

	return response.StatusCode
}

func stopLRP(processGUID, versionGUID string) (*http.Response, error) {
	request, err := http.NewRequest("PUT", fmt.Sprintf("%s/apps/%s/%s/stop", tests.GetEiriniAddress(), processGUID, versionGUID), nil)
	if err != nil {
		return nil, err
	}

	return fixture.GetEiriniHTTPClient().Do(request)
}

func stopLRPInstance(processGUID, versionGUID string, instance int) (*http.Response, error) {
	request, err := http.NewRequest("PUT", fmt.Sprintf("%s/apps/%s/%s/stop/%d", tests.GetEiriniAddress(), processGUID, versionGUID, instance), nil)
	if err != nil {
		return nil, err
	}

	return fixture.GetEiriniHTTPClient().Do(request)
}

func updateLRP(updateRequest cf.UpdateDesiredLRPRequest) (*http.Response, error) {
	body, err := json.Marshal(updateRequest)
	if err != nil {
		return nil, err
	}

	updateLrpReq, err := http.NewRequest("POST", fmt.Sprintf("%s/apps/%s", tests.GetEiriniAddress(), updateRequest.GUID), bytes.NewReader(body))
	if err != nil {
		return nil, err
	}

	return fixture.GetEiriniHTTPClient().Do(updateLrpReq)
}

func desireTask(taskRequest cf.TaskRequest) {
	data, err := json.Marshal(taskRequest)
	Expect(err).NotTo(HaveOccurred())

	request, err := http.NewRequest("POST", fmt.Sprintf("%s/tasks/%s", tests.GetEiriniAddress(), taskRequest.GUID), bytes.NewReader(data))
	Expect(err).NotTo(HaveOccurred())

	response, err := fixture.GetEiriniHTTPClient().Do(request)
	Expect(err).NotTo(HaveOccurred())

	defer response.Body.Close()

	Expect(response).To(HaveHTTPStatus(http.StatusAccepted))
}

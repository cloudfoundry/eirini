package eats_test

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"

	"code.cloudfoundry.org/eirini/k8s/stset"
	"code.cloudfoundry.org/eirini/models/cf"
	"code.cloudfoundry.org/eirini/tests"
	"code.cloudfoundry.org/eirini/tests/eats/wiremock"
	"code.cloudfoundry.org/runtimeschema/cc_messages"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

var _ = Describe("EventsReporter [needs-logs-for: eirini-api, eirini-event-reporter]", func() {
	var (
		guid            string
		version         string
		appServiceName  string
		expectedRequest wiremock.RequestMatcher
	)

	BeforeEach(func() {
		guid = tests.GenerateGUID()
		version = tests.GenerateGUID()

		expectedRequest = wiremock.RequestMatcher{
			Method: "POST",
			URL:    fmt.Sprintf("/internal/v4/apps/%s-%s/crashed", guid, version),
		}
		err := fixture.Wiremock.AddStub(wiremock.Stub{
			Request: expectedRequest,
			Response: wiremock.Response{
				Status: 200,
			},
		})
		Expect(err).NotTo(HaveOccurred())
	})

	AfterEach(func() {
		_, err := stopLRP(guid, version)
		Expect(err).NotTo(HaveOccurred())
	})

	When("the app starts successfully", func() {
		BeforeEach(func() {
			statusCode := desireLRP(cf.DesireLRPRequest{
				Namespace:    fixture.Namespace,
				GUID:         guid,
				Version:      version,
				NumInstances: 1,
				DiskMB:       512,
				Lifecycle: cf.Lifecycle{
					DockerLifecycle: &cf.DockerLifecycle{
						Image:            "eirini/dorini",
						RegistryUsername: "eiriniuser",
						RegistryPassword: tests.GetEiriniDockerHubPassword(),
					},
				},
			})
			Expect(statusCode).To(Equal(http.StatusAccepted))

			appServiceName = exposeLRP(fixture.Namespace, guid, 8080, "/")
		})

		AfterEach(func() {
			unexposeLRP(fixture.Namespace, appServiceName)
		})

		It("does not report a crash event for running apps", func() {
			Consistently(fixture.Wiremock.GetCountFn(expectedRequest), "10s").Should(BeZero())
		})

		When("the app exits with non-zero code", func() {
			BeforeEach(func() {
				_, err := http.Get(fmt.Sprintf("http://%s.%s:8080/exit?exitCode=1", appServiceName, fixture.Namespace))
				Expect(err).To(MatchError(ContainSubstring("EOF"))) // The app exited
			})

			It("reports a crash event", func() {
				Eventually(fixture.Wiremock.GetCountFn(expectedRequest)).Should(Equal(1))
				Consistently(fixture.Wiremock.GetCountFn(expectedRequest), "10s").Should(Equal(1))

				verifyCrashRequest(expectedRequest, 1)
			})
		})

		When("the app exits with zero code", func() {
			BeforeEach(func() {
				_, err := http.Get(fmt.Sprintf("http://%s.%s:8080/exit?exitCode=0", appServiceName, fixture.Namespace))
				Expect(err).To(MatchError(ContainSubstring("EOF"))) // The app exited
			})

			It("reports a crash event", func() {
				Eventually(fixture.Wiremock.GetCountFn(expectedRequest)).Should(Equal(1))
				Consistently(fixture.Wiremock.GetCountFn(expectedRequest), "10s").Should(Equal(1))

				verifyCrashRequest(expectedRequest, 0)
			})
		})
	})

	When("the app is crashing on startup", func() {
		BeforeEach(func() {
			_, err := stopLRP(guid, version)
			Expect(err).NotTo(HaveOccurred())

			statusCode := desireLRP(cf.DesireLRPRequest{
				Namespace:    fixture.Namespace,
				GUID:         guid,
				Version:      version,
				NumInstances: 1,
				DiskMB:       512,
				Lifecycle: cf.Lifecycle{
					DockerLifecycle: &cf.DockerLifecycle{
						Image:            "eirini/busybox",
						Command:          []string{"bad command"},
						RegistryUsername: "eiriniuser",
						RegistryPassword: tests.GetEiriniDockerHubPassword(),
					},
				},
			})
			Expect(statusCode).To(Equal(http.StatusAccepted))
		})

		It("reports a crash event per app restart", func() {
			Eventually(fixture.Wiremock.GetCountFn(expectedRequest)).Should(BeNumerically(">", 1))
		})
	})
})

func exposeLRP(namespace, guid string, appPort int32, pingPath ...string) string {
	service, err := fixture.Clientset.CoreV1().Services(namespace).Create(context.Background(), &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name: "service-" + guid,
		},
		Spec: corev1.ServiceSpec{
			Ports: []corev1.ServicePort{
				{
					Port: appPort,
				},
			},
			Selector: map[string]string{
				stset.LabelGUID: guid,
			},
		},
	}, metav1.CreateOptions{})
	ExpectWithOffset(1, err).NotTo(HaveOccurred())

	if len(pingPath) > 0 {
		pingURL := &url.URL{
			Scheme: "http",
			Host:   fmt.Sprintf("%s.%s:%d", service.Name, namespace, appPort),
			Path:   pingPath[0],
		}

		EventuallyWithOffset(1, func() error {
			resp, err := http.Get(pingURL.String())
			if err != nil {
				return err
			}
			defer resp.Body.Close()

			if resp.StatusCode != http.StatusOK {
				return fmt.Errorf("request failed: %s", resp.Status)
			}

			return nil
		}).Should(Succeed())
	}

	return service.Name
}

func unexposeLRP(namespace, serviceName string) {
	ExpectWithOffset(1, fixture.Clientset.CoreV1().Services(namespace).Delete(context.Background(), serviceName, metav1.DeleteOptions{})).To(Succeed())
}

func verifyCrashRequest(requestMatcher wiremock.RequestMatcher, exitStatus int) {
	body, err := fixture.Wiremock.GetRequestBody(requestMatcher)
	ExpectWithOffset(1, err).NotTo(HaveOccurred())

	var request cc_messages.AppCrashedRequest
	err = json.Unmarshal([]byte(body), &request)
	ExpectWithOffset(1, err).NotTo(HaveOccurred())

	ExpectWithOffset(1, request.ExitStatus).To(Equal(exitStatus))
	ExpectWithOffset(1, request.CrashCount).To(Equal(1))
}

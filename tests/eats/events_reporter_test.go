package eats_test

import (
	"context"
	"fmt"
	"net/http"
	"net/url"

	"code.cloudfoundry.org/eirini/k8s"
	"code.cloudfoundry.org/eirini/models/cf"
	"code.cloudfoundry.org/eirini/tests"
	"code.cloudfoundry.org/eirini/tests/eats/wiremock"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

var _ = Describe("EventsReporter", func() {
	var (
		guid           string
		version        string
		appServiceName string
	)

	BeforeEach(func() {
		guid = tests.GenerateGUID()
		version = tests.GenerateGUID()

		err := fixture.Wiremock.AddStub(wiremock.Stub{
			Request: wiremock.RequestMatcher{
				Method: "POST",
				URL:    fmt.Sprintf("/internal/v4/apps/%s-%s/crashed", guid, version),
			},
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
						Image:            "eiriniuser/notdora",
						RegistryUsername: "eiriniuser",
						RegistryPassword: tests.GetEiriniDockerHubPassword(),
					},
				},
			})
			Expect(statusCode).To(Equal(http.StatusAccepted))

			appServiceName = exposeLRP(fixture.Namespace, guid, 8888, "/")
		})

		AfterEach(func() {
			unexposeLRP(fixture.Namespace, appServiceName)
		})

		It("does not report a crash event for running apps", func() {
			requestMatcher := wiremock.RequestMatcher{
				Method: "POST",
				URL:    fmt.Sprintf("/internal/v4/apps/%s-1/crashed", guid),
			}

			Consistently(fixture.Wiremock.GetCountFn(requestMatcher), "10s").Should(BeZero())
		})

		When("the app exists with non-zero code", func() {
			BeforeEach(func() {
				_, err := http.Get(fmt.Sprintf("http://%s.%s:8888/exit?exitCode=1", appServiceName, fixture.Namespace))
				Expect(err).To(MatchError(ContainSubstring("EOF"))) // The app exited
			})

			It("reports a crash event", func() {
				requestMatcher := wiremock.RequestMatcher{
					Method: "POST",
					URL:    fmt.Sprintf("/internal/v4/apps/%s-%s/crashed", guid, version),
				}

				Eventually(fixture.Wiremock.GetCountFn(requestMatcher)).Should(Equal(1))
				Consistently(fixture.Wiremock.GetCountFn(requestMatcher), "10s").Should(Equal(1))
			})
		})

		When("the app exists with zero code", func() {
			BeforeEach(func() {
				_, err := http.Get(fmt.Sprintf("http://%s.%s:8888/exit?exitCode=0", appServiceName, fixture.Namespace))
				Expect(err).To(MatchError(ContainSubstring("EOF"))) // The app exited
			})

			It("reports a crash event", func() {
				requestMatcher := wiremock.RequestMatcher{
					Method: "POST",
					URL:    fmt.Sprintf("/internal/v4/apps/%s-%s/crashed", guid, version),
				}

				Eventually(fixture.Wiremock.GetCountFn(requestMatcher)).Should(Equal(1))
				Consistently(fixture.Wiremock.GetCountFn(requestMatcher), "10s").Should(Equal(1))
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
			requestMatcher := wiremock.RequestMatcher{
				Method: "POST",
				URL:    fmt.Sprintf("/internal/v4/apps/%s-%s/crashed", guid, version),
			}

			Eventually(fixture.Wiremock.GetCountFn(requestMatcher)).Should(BeNumerically(">", 1))
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
				k8s.LabelGUID: guid,
			},
		},
	}, metav1.CreateOptions{})
	Expect(err).NotTo(HaveOccurred())

	if len(pingPath) > 0 {
		pingURL := &url.URL{
			Scheme: "http",
			Host:   fmt.Sprintf("%s.%s:%d", service.Name, namespace, appPort),
			Path:   pingPath[0],
		}

		Eventually(func() error {
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
	Expect(fixture.Clientset.CoreV1().Services(namespace).Delete(context.Background(), serviceName, metav1.DeleteOptions{})).To(Succeed())
}

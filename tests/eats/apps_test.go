package eats_test

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"

	"code.cloudfoundry.org/eirini/k8s"
	"code.cloudfoundry.org/eirini/models/cf"
	"code.cloudfoundry.org/eirini/tests"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gstruct"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

var _ = Describe("Apps", func() {
	var (
		namespace      string
		lrpGUID        string
		lrpVersion     string
		lrpProcessGUID string
	)

	BeforeEach(func() {
		namespace = fixture.Namespace
		lrpGUID = tests.GenerateGUID()
		lrpVersion = tests.GenerateGUID()
		lrpProcessGUID = processGUID(lrpGUID, lrpVersion)
	})

	Describe("Desiring an app", func() {
		var desireRespStatusCode int

		JustBeforeEach(func() {
			desireRespStatusCode = desireApp(lrpGUID, lrpVersion, namespace)
		})

		AfterEach(func() {
			_, err := stopLRP(lrpGUID, lrpVersion)
			Expect(err).NotTo(HaveOccurred())
		})

		It("succeeds", func() {
			Expect(desireRespStatusCode).To(Equal(http.StatusAccepted))
		})

		It("deploys the LRP to the specified namespace", func() {
			Expect(getStatefulSet(lrpGUID, lrpVersion).Namespace).To(Equal(fixture.Namespace))
			Eventually(func() bool {
				return getPodReadiness(lrpGUID, lrpVersion)
			}).Should(BeTrue(), "LRP Pod not ready")
		})

		When("a namespace is not specified", func() {
			BeforeEach(func() {
				namespace = ""
			})

			It("deploys the LRP to the workloads namespace", func() {
				Expect(getStatefulSet(lrpGUID, lrpVersion).Namespace).To(Equal(fixture.GetEiriniWorkloadsNamespace()))
			})
		})

		When("the app already exist", func() {
			It("returns 202", func() {
				respStatusCode := desireApp(lrpGUID, lrpVersion, namespace)
				Expect(respStatusCode).To(Equal(http.StatusAccepted))
			})
		})
	})

	Describe("Update an app", func() {
		var (
			desiredLRPUpdate cf.DesiredLRPUpdate
			lrp              cf.DesiredLRP
		)

		BeforeEach(func() {
			desireApp(lrpGUID, lrpVersion, namespace)
		})

		JustBeforeEach(func() {
			updateResp, err := updateLRP(cf.UpdateDesiredLRPRequest{
				GUID:    lrpGUID,
				Version: lrpVersion,
				Update:  desiredLRPUpdate,
			})
			Expect(err).NotTo(HaveOccurred())
			Expect(updateResp.StatusCode).To(Equal(http.StatusOK))

			lrp, err = getLRP(lrpGUID, lrpVersion)
			Expect(err).NotTo(HaveOccurred())
		})

		When("updating the instance number", func() {
			BeforeEach(func() {
				desiredLRPUpdate = cf.DesiredLRPUpdate{
					Instances: 2,
				}
			})

			It("successfully updates the LRP", func() {
				Expect(lrp.Instances).To(BeNumerically("==", 2))
			})
		})

		When("updating the routes", func() {
			var updatedRoutes []tests.RouteInfo

			BeforeEach(func() {
				updatedRoutes = []tests.RouteInfo{{Hostname: "updated-host", Port: 4321}}
				desiredLRPUpdate = cf.DesiredLRPUpdate{
					Routes: map[string]json.RawMessage{
						"cf-router": tests.MarshalRoutes(updatedRoutes),
					},
				}
			})

			It("successfully updates the LRP", func() {
				Expect(lrp.Routes).To(SatisfyAll(
					HaveLen(1),
					HaveKeyWithValue("cf-router", tests.MarshalRoutes(updatedRoutes))),
				)
			})
		})

		When("updating the annotation", func() {
			BeforeEach(func() {
				desiredLRPUpdate = cf.DesiredLRPUpdate{
					Annotation: "333333",
				}
			})

			It("successfully updates the LRP", func() {
				Expect(lrp.Annotation).To(Equal("333333"))
			})
		})

		When("updating the image", func() {
			BeforeEach(func() {
				desiredLRPUpdate = cf.DesiredLRPUpdate{
					Image: "new/image",
				}
			})

			It("successfully updates the LRP", func() {
				Expect(lrp.Image).To(Equal("new/image"))
			})
		})
	})

	Describe("Listing apps", func() {
		var (
			anotherLrpGUID    string
			anotherLrpVersion string
		)

		JustBeforeEach(func() {
			anotherLrpGUID = tests.GenerateGUID()
			anotherLrpVersion = tests.GenerateGUID()

			firstLrp := createLrpRequest(lrpGUID, lrpVersion)
			firstLrp.NumInstances = 2
			firstLrp.LastUpdated = "111111"
			firstLrp.Namespace = namespace
			desireLRP(firstLrp)

			secondLrp := createLrpRequest(anotherLrpGUID, anotherLrpVersion)
			secondLrp.LastUpdated = "222222"
			secondLrp.Namespace = namespace
			desireLRP(secondLrp)
		})

		It("successfully lists all LRPs", func() {
			lrps, err := getLRPs()
			Expect(err).NotTo(HaveOccurred())

			lrpsAnnotations := make(map[string]string)
			for _, lrp := range lrps {
				lrpsAnnotations[lrp.DesiredLRPKey.ProcessGUID] = lrp.Annotation
			}

			anotherProcessGUID := processGUID(anotherLrpGUID, anotherLrpVersion)

			Expect(lrpsAnnotations).To(SatisfyAll(HaveKey(lrpProcessGUID), HaveKey(anotherProcessGUID)))
			Expect(lrpsAnnotations[lrpProcessGUID]).To(Equal("111111"))
			Expect(lrpsAnnotations[anotherProcessGUID]).To(Equal("222222"))
		})

		When("non-eirini statefulSets exist", func() {
			BeforeEach(func() {
				_, err := fixture.Clientset.AppsV1().StatefulSets(fixture.Namespace).Create(context.Background(), &appsv1.StatefulSet{
					ObjectMeta: metav1.ObjectMeta{
						Name: tests.GenerateGUID(),
					},
					Spec: appsv1.StatefulSetSpec{
						Template: corev1.PodTemplateSpec{
							ObjectMeta: metav1.ObjectMeta{
								Labels: map[string]string{"foo": "bar"},
							},
						},
						Selector: &metav1.LabelSelector{
							MatchLabels: map[string]string{"foo": "bar"},
						},
					},
				}, metav1.CreateOptions{})
				Expect(err).NotTo(HaveOccurred())
			})

			It("does not list them", func() {
				lrps, err := getLRPs()
				Expect(err).NotTo(HaveOccurred())

				for _, lrp := range lrps {
					Expect(lrp.GUID).NotTo(BeEmpty(), fmt.Sprintf("%#v does not look like an LRP", lrp))
					Expect(lrp.Version).NotTo(BeEmpty(), fmt.Sprintf("%#v does not look like an LRP", lrp))
				}
			})
		})
	})

	Describe("Get an app", func() {
		JustBeforeEach(func() {
			desireApp(lrpGUID, lrpVersion, namespace)
		})

		It("successfully returns the LRP", func() {
			lrp, err := getLRP(lrpGUID, lrpVersion)
			Expect(err).NotTo(HaveOccurred())
			Expect(lrp.ProcessGUID).To(Equal(lrpProcessGUID))
			Expect(lrp.Instances).To(Equal(int32(1)))
		})

		When("the app doesn't exist", func() {
			It("returns a 404", func() {
				_, err := getLRP("does-not-exist", lrpVersion)
				Expect(err).To(MatchError("404 Not Found"))
			})
		})
	})

	Describe("Stop an app", func() {
		BeforeEach(func() {
			desireApp(lrpGUID, lrpVersion, namespace)
		})

		It("successfully stops the app", func() {
			_, err := stopLRP(lrpGUID, lrpVersion)
			Expect(err).NotTo(HaveOccurred())
			_, err = getLRP(lrpGUID, lrpVersion)
			Expect(err).To(MatchError("404 Not Found"))
		})

		When("the app doesn't exist", func() {
			It("succeeds", func() {
				response, err := stopLRP("does-not-exist", lrpVersion)
				Expect(err).NotTo(HaveOccurred())
				Expect(response.StatusCode).To(Equal(http.StatusOK))
			})
		})
	})

	Describe("Stop an app instance", func() {
		BeforeEach(func() {
			desireAppWithInstances(lrpGUID, lrpVersion, namespace, 3)
			Eventually(func() []*cf.Instance {
				return getRunningInstances(lrpGUID, lrpVersion)
			}).Should(HaveLen(3))
		})

		It("successfully stops the instance", func() {
			_, err := stopLRPInstance(lrpGUID, lrpVersion, 1)
			Expect(err).NotTo(HaveOccurred())

			Eventually(func() []*cf.Instance {
				return getRunningInstances(lrpGUID, lrpVersion)
			}).Should(ConsistOf(
				gstruct.PointTo(gstruct.MatchFields(gstruct.IgnoreExtras, gstruct.Fields{
					"Index": Equal(0),
					"State": Equal("RUNNING"),
					"Since": BeNumerically(">", 0),
				})),
				gstruct.PointTo(gstruct.MatchFields(gstruct.IgnoreExtras, gstruct.Fields{
					"Index": Equal(2),
					"State": Equal("RUNNING"),
					"Since": BeNumerically(">", 0),
				})),
			))
		})

		When("the app does not exist", func() {
			It("succeeds", func() {
				resp, err := stopLRPInstance("does-not-exist", lrpVersion, 1)
				Expect(err).NotTo(HaveOccurred())
				Expect(resp.StatusCode).To(Equal(http.StatusOK))
			})
		})

		When("the app instance does not exist", func() {
			It("should return 400", func() {
				resp, err := stopLRPInstance(lrpGUID, lrpVersion, 99)
				Expect(err).NotTo(HaveOccurred())
				Expect(resp.StatusCode).To(Equal(http.StatusBadRequest))
			})
		})

		When("the app instance is a negative number", func() {
			It("should return 400", func() {
				resp, err := stopLRPInstance(lrpGUID, lrpVersion, -1)
				Expect(err).NotTo(HaveOccurred())
				Expect(resp.StatusCode).To(Equal(http.StatusBadRequest))
			})
		})
	})

	Describe("Get instances", func() {
		JustBeforeEach(func() {
			desireAppWithInstances(lrpGUID, lrpVersion, namespace, 3)
		})

		It("returns the app instances", func() {
			var resp *cf.GetInstancesResponse
			Eventually(func() []*cf.Instance {
				var err error
				resp, err = getInstances(lrpGUID, lrpVersion)
				Expect(err).NotTo(HaveOccurred())

				return resp.Instances
			}).Should(ConsistOf(
				gstruct.PointTo(gstruct.MatchFields(gstruct.IgnoreExtras, gstruct.Fields{
					"Index": Equal(0),
					"State": Equal("RUNNING"),
					"Since": BeNumerically(">", 0),
				})),
				gstruct.PointTo(gstruct.MatchFields(gstruct.IgnoreExtras, gstruct.Fields{
					"Index": Equal(1),
					"State": Equal("RUNNING"),
					"Since": BeNumerically(">", 0),
				})),
				gstruct.PointTo(gstruct.MatchFields(gstruct.IgnoreExtras, gstruct.Fields{
					"Index": Equal(2),
					"State": Equal("RUNNING"),
					"Since": BeNumerically(">", 0),
				})),
			))
		})

		When("the app doesn't exist", func() {
			It("returns a 404", func() {
				resp, err := getInstances("does-not-exist", lrpVersion)
				Expect(err).To(MatchError("404 Not Found"))
				Expect(resp.Error).To(Equal("failed to get instances for app: not found"))
			})
		})
	})
})

func desireAppWithInstances(appGUID, version, namespace string, instances int) {
	lrp := createLrpRequest(appGUID, version)
	lrp.NumInstances = instances
	lrp.Namespace = namespace

	desireLRP(lrp)
}

func desireApp(appGUID, version, namespace string) int {
	lrp := createLrpRequest(appGUID, version)
	lrp.Namespace = namespace

	return desireLRP(lrp)
}

func createLrpRequest(appGUID, version string) cf.DesireLRPRequest {
	return cf.DesireLRPRequest{
		GUID:         appGUID,
		Version:      version,
		NumInstances: 1,
		Ports:        []int32{8080},
		DiskMB:       100,
		Lifecycle: cf.Lifecycle{
			DockerLifecycle: &cf.DockerLifecycle{
				Image: "eirini/dorini",
			},
		},
	}
}

func getRunningInstances(appGUID, version string) []*cf.Instance {
	instancesResp, err := getInstances(appGUID, version)
	Expect(err).NotTo(HaveOccurred())

	runningInstances := []*cf.Instance{}

	for _, i := range instancesResp.Instances {
		if i.State == "RUNNING" {
			runningInstances = append(runningInstances, i)
		}
	}

	return runningInstances
}

func processGUID(guid, version string) string {
	return fmt.Sprintf("%s-%s", guid, version)
}

func getStatefulSet(guid, version string) *appsv1.StatefulSet {
	statefulSets, err := fixture.Clientset.AppsV1().StatefulSets("").List(context.Background(), metav1.ListOptions{
		LabelSelector: fmt.Sprintf("%s=%s,%s=%s", k8s.LabelGUID, guid, k8s.LabelVersion, version),
	})
	Expect(err).NotTo(HaveOccurred())
	Expect(statefulSets.Items).To(HaveLen(1))

	return &statefulSets.Items[0]
}

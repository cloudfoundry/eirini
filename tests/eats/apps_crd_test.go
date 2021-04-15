package eats_test

import (
	"context"
	"fmt"
	"time"

	"code.cloudfoundry.org/eirini/pkg/apis/eirini"
	eiriniv1 "code.cloudfoundry.org/eirini/pkg/apis/eirini/v1"
	"code.cloudfoundry.org/eirini/prometheus"
	"code.cloudfoundry.org/eirini/tests"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/prometheus/client_golang/api"
	prometheusv1 "github.com/prometheus/client_golang/api/prometheus/v1"
	"github.com/prometheus/common/model"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

var _ = Describe("Apps CRDs [needs-logs-for: eirini-api, eirini-controller]", func() {
	var (
		lrpName          string
		lrpGUID          string
		lrpVersion       string
		lrp              *eiriniv1.LRP
		prometheusClient api.Client
		prometheusAPI    prometheusv1.API
	)

	getLRP := func() *eiriniv1.LRP {
		l, err := fixture.EiriniClientset.
			EiriniV1().
			LRPs(fixture.Namespace).
			Get(context.Background(), lrpName, metav1.GetOptions{})

		Expect(err).NotTo(HaveOccurred())

		return l
	}

	getMetric := func(metric, name string) (int, error) {
		result, _, err := prometheusAPI.Query(context.Background(), fmt.Sprintf(`%s{name="%s"} > 0`, metric, name), time.Now())
		if err != nil {
			return 0, err
		}

		resultVector, ok := result.(model.Vector)
		if !ok {
			return 0, fmt.Errorf("result is not a vector: %+v", result)
		}

		if len(resultVector) == 0 {
			return 0, nil
		}

		if len(resultVector) > 1 {
			return 0, fmt.Errorf("result vector contains multiple values: %+v", resultVector)
		}

		return int(resultVector[0].Value), nil
	}

	getMetricFn := func(metric, name string) func() (int, error) {
		return func() (int, error) {
			return getMetric(metric, name)
		}
	}

	BeforeEach(func() {
		lrpName = tests.GenerateGUID()
		lrpGUID = tests.GenerateGUID()
		lrpVersion = tests.GenerateGUID()

		var connErr error
		prometheusClient, connErr = api.NewClient(api.Config{
			Address: fmt.Sprintf("http://prometheus-server.%s.svc.cluster.local:80", tests.GetEiriniSystemNamespace()),
		})
		Expect(connErr).NotTo(HaveOccurred())
		prometheusAPI = prometheusv1.NewAPI(prometheusClient)

		lrp = &eiriniv1.LRP{
			ObjectMeta: metav1.ObjectMeta{
				Name: lrpName,
			},
			Spec: eiriniv1.LRPSpec{
				GUID:                   lrpGUID,
				Version:                lrpVersion,
				Image:                  "eirini/dorini",
				AppGUID:                "the-app-guid",
				AppName:                "k-2so",
				SpaceName:              "s",
				OrgName:                "o",
				Env:                    map[string]string{"FOO": "BAR"},
				MemoryMB:               256,
				DiskMB:                 256,
				CPUWeight:              10,
				Instances:              1,
				LastUpdated:            "a long time ago in a galaxy far, far away",
				Ports:                  []int32{8080},
				VolumeMounts:           []eiriniv1.VolumeMount{},
				UserDefinedAnnotations: map[string]string{},
				AppRoutes:              []eiriniv1.Route{{Hostname: "app-hostname-1", Port: 8080}},
			},
		}
	})

	AfterEach(func() {
		bgDelete := metav1.DeletePropagationBackground
		err := fixture.EiriniClientset.
			EiriniV1().
			LRPs(fixture.Namespace).
			DeleteCollection(context.Background(),
				metav1.DeleteOptions{PropagationPolicy: &bgDelete},
				metav1.ListOptions{FieldSelector: "metadata.name=" + lrpName},
			)
		Expect(err).NotTo(HaveOccurred())
	})

	Describe("Desiring an app", func() {
		var (
			clientErr      error
			appServiceName string
		)

		JustBeforeEach(func() {
			_, clientErr = fixture.EiriniClientset.
				EiriniV1().
				LRPs(fixture.Namespace).
				Create(context.Background(), lrp, metav1.CreateOptions{})

			appServiceName = exposeLRP(fixture.Namespace, lrpGUID, 8080)
		})

		It("succeeds", func() {
			Expect(clientErr).NotTo(HaveOccurred())
		})

		It("starts the app", func() {
			Eventually(pingLRPFn(fixture.Namespace, appServiceName, 8080, "/")).Should(ContainSubstring("Hi, I'm not Dora!"))
		})

		It("updates the CRD status", func() {
			Eventually(func() int32 {
				return getLRP().Status.Replicas
			}).Should(Equal(int32(1)))
		})

		Describe("Prometheus metrics", func() {
			var (
				creationsBefore              int
				creationDurationSumsBefore   int
				creationDurationCountsBefore int
				err                          error
			)

			BeforeEach(func() {
				creationsBefore, err = getMetric(prometheus.LRPCreations, "eirini-controller")
				Expect(err).NotTo(HaveOccurred())
				creationDurationSumsBefore, err = getMetric(prometheus.LRPCreationDurations+"_sum", "eirini-controller")
				Expect(err).NotTo(HaveOccurred())
				creationDurationCountsBefore, err = getMetric(prometheus.LRPCreationDurations+"_count", "eirini-controller")
				Expect(err).NotTo(HaveOccurred())
			})

			It("increments the created LRP counter", func() {
				Eventually(getMetricFn(prometheus.LRPCreations, "eirini-controller"), "1m").
					Should(BeNumerically(">", creationsBefore))
			})

			It("observes the creation duration", func() {
				Eventually(getMetricFn(prometheus.LRPCreationDurations+"_sum", "eirini-controller"), "1m").
					Should(BeNumerically(">", creationDurationSumsBefore))
				Eventually(getMetricFn(prometheus.LRPCreationDurations+"_count", "eirini-controller"), "1m").
					Should(BeNumerically(">", creationDurationCountsBefore))
			})
		})

		When("the disk quota is not specified", func() {
			It("fails", func() {
				obj := &unstructured.Unstructured{
					Object: map[string]interface{}{
						"kind":       "LRP",
						"apiVersion": "eirini.cloudfoundry.org/v1",
						"metadata": map[string]interface{}{
							"name": "the-invalid-lrp",
						},
						"spec": map[string]interface{}{
							"guid":      lrpGUID,
							"version":   lrpVersion,
							"image":     "eirini/dorini",
							"appGUID":   "the-app-guid",
							"appName":   "k-2so",
							"spaceName": "s",
							"orgName":   "o",
							"env":       map[string]string{"FOO": "BAR"},
							"instances": 1,
							"appRoutes": []eiriniv1.Route{{Hostname: "app-hostname-1", Port: 8080}},
						},
					},
				}
				_, err := fixture.DynamicClientset.
					Resource(schema.GroupVersionResource{
						Group:    eirini.GroupName,
						Version:  "v1",
						Resource: "lrps",
					}).
					Namespace(fixture.Namespace).
					Create(context.Background(), obj, metav1.CreateOptions{})
				Expect(err).To(MatchError(ContainSubstring("diskMB: Required value")))
			})
		})

		When("the disk quota is 0", func() {
			BeforeEach(func() {
				lrp.Spec.DiskMB = 0
			})

			It("fails", func() {
				Expect(clientErr).To(MatchError(ContainSubstring("spec.diskMB in body should be greater than or equal to 1")))
			})
		})
	})

	Describe("Update an app", func() {
		var clientErr error

		BeforeEach(func() {
			_, err := fixture.EiriniClientset.
				EiriniV1().
				LRPs(fixture.Namespace).
				Create(context.Background(), lrp, metav1.CreateOptions{})
			Expect(err).NotTo(HaveOccurred())

			Eventually(func() int32 {
				lrp = getLRP()

				return lrp.Status.Replicas
			}).Should(Equal(int32(1)))
		})

		JustBeforeEach(func() {
			_, clientErr = fixture.EiriniClientset.
				EiriniV1().
				LRPs(fixture.Namespace).
				Update(context.Background(), lrp, metav1.UpdateOptions{})
		})

		When("updating the instance count", func() {
			BeforeEach(func() {
				lrp.Spec.Instances = 3
			})

			It("succeeds", func() {
				Expect(clientErr).NotTo(HaveOccurred())
			})

			It("updates the LRP Status replicas", func() {
				Eventually(func() int {
					return int(getLRP().Status.Replicas)
				}).Should(Equal(3))
			})
		})

		When("updating an immutable property", func() {
			BeforeEach(func() {
				lrp.Spec.Command = []string{"you", "shall", "not", "pass"}
			})

			It("fails", func() {
				Expect(clientErr).To(MatchError(ContainSubstring("Changing immutable fields not allowed: Command")))
			})
		})
	})

	Describe("Stop an app", func() {
		var appServiceName string

		BeforeEach(func() {
			_, err := fixture.EiriniClientset.
				EiriniV1().
				LRPs(fixture.Namespace).
				Create(context.Background(), lrp, metav1.CreateOptions{})
			Expect(err).NotTo(HaveOccurred())

			appServiceName = exposeLRP(fixture.Namespace, lrpGUID, 8080, "/")
		})

		JustBeforeEach(func() {
			Expect(fixture.EiriniClientset.
				EiriniV1().
				LRPs(fixture.Namespace).
				Delete(context.Background(), lrpName, metav1.DeleteOptions{}),
			).To(Succeed())
		})

		It("should stop", func() {
			Eventually(func() error {
				_, err := pingLRPFn(fixture.Namespace, appServiceName, 8080, "/")()

				return err
			}).Should(HaveOccurred())

			Consistently(func() error {
				_, err := pingLRPFn(fixture.Namespace, appServiceName, 8080, "/")()

				return err
			}, "2s").Should(HaveOccurred())
		})
	})
})

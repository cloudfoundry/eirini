package main_test

import (
	"time"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/pkg/errors"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	. "code.cloudfoundry.org/eirini/cmd/bits-waiter"
	"code.cloudfoundry.org/eirini/models/cf"
	"code.cloudfoundry.org/eirini/rootfspatcher/rootfspatcherfakes"
	"code.cloudfoundry.org/lager/lagertest"
)

var _ = Describe("StatefulsetWaiter", func() {

	var (
		fakeLister *rootfspatcherfakes.FakeDeploymentLister
		waiter     DeploymentWaiter
		logger     *lagertest.TestLogger
	)

	BeforeEach(func() {
		fakeLister = new(rootfspatcherfakes.FakeDeploymentLister)
		logger = lagertest.NewTestLogger("test")
		waiter = DeploymentWaiter{
			Deployments:       fakeLister,
			Timeout:           1 * time.Second,
			Logger:            logger,
			ListLabelSelector: "my=label",
		}
	})

	It("should wait for all statefulsets to be updated and ready", func() {
		ssList := &appsv1.DeploymentList{
			Items: []appsv1.Deployment{createStatefulSet("name", "version", replicaStatuses{
				desired: 1,
				ready:   1,
				current: 1,
				updated: 1,
			})},
		}
		fakeLister.ListReturns(ssList, nil)

		err := waiter.Wait([]string{})
		Expect(err).ToNot(HaveOccurred())
	})

	It("should wait until statefulsets status is tracking the updated pods", func() {
		outdatedSS := createStatefulSet("name", "version", replicaStatuses{
			desired: 1,
			ready:   1,
			current: 1,
			updated: 1,
		})
		outdatedSS.Generation = 3
		outdatedSS.Status.ObservedGeneration = 2

		updatedSS := createStatefulSet("name", "version", replicaStatuses{
			desired: 1,
			ready:   1,
			current: 1,
			updated: 1,
		})
		updatedSS.Generation = 3
		updatedSS.Status.ObservedGeneration = 3

		fakeLister.ListReturnsOnCall(0, &appsv1.StatefulSetList{Items: []appsv1.StatefulSet{outdatedSS}}, nil)
		fakeLister.ListReturnsOnCall(1, &appsv1.StatefulSetList{Items: []appsv1.StatefulSet{updatedSS}}, nil)

		waiter.Timeout = 2 * time.Second
		err := waiter.Wait([]string{})
		Expect(err).ToNot(HaveOccurred())
		Expect(fakeLister.ListCallCount()).To(Equal(2))
	})

	When("statefulsets don't become ready before timeout", func() {
		When("there are non-ready replicas", func() {
			BeforeEach(func() {
				ssList := &appsv1.StatefulSetList{
					Items: []appsv1.StatefulSet{createStatefulSet("name", "version", replicaStatuses{
						desired: 3,
						ready:   1,
					})},
				}
				fakeLister.ListReturns(ssList, nil)

			})

			It("should return error", func() {
				Expect(waiter.Wait([]string{})).To(MatchError("timed out after 1s"))
			})
		})

		When("there are non-updated replicas", func() {
			BeforeEach(func() {
				ssList := &appsv1.StatefulSetList{
					Items: []appsv1.StatefulSet{createStatefulSet("name", "version", replicaStatuses{
						desired: 3,
						ready:   3,
						updated: 2,
					})},
				}
				fakeLister.ListReturns(ssList, nil)

			})

			It("should return error", func() {
				Expect(waiter.Wait([]string{})).To(MatchError("timed out after 1s"))
			})
		})

		When("current replicas are less than desired", func() {
			BeforeEach(func() {
				ssList := &appsv1.StatefulSetList{
					Items: []appsv1.StatefulSet{createStatefulSet("name", "version", replicaStatuses{
						desired: 3,
						ready:   3,
						updated: 3,
						current: 1,
					})},
				}
				fakeLister.ListReturns(ssList, nil)

			})

			It("should return error", func() {
				Expect(waiter.Wait([]string{})).To(MatchError("timed out after 1s"))
			})
		})

		When("they are ignored", func() {

			It("should return normally", func() {
				failingStatefulSet := createStatefulSet("name", "version", replicaStatuses{
					desired: 3,
					ready:   1,
				})
				ssList := &appsv1.StatefulSetList{
					Items: []appsv1.StatefulSet{failingStatefulSet},
				}
				fakeLister.ListReturns(ssList, nil)

				ignored := []string{
					failingStatefulSet.Annotations[cf.ProcessGUID],
				}
				err := waiter.Wait(ignored)
				Expect(err).ToNot(HaveOccurred())
			})
		})
	})

	When("pods don't match the expected labels", func() {
		Context("key is missing", func() {
			BeforeEach(func() {
				waiter.ExpectedPodLabels = map[string]string{
					"foo": "bar",
					"baz": "wat",
				}

				statefulset := createStatefulSet("name", "version", replicaStatuses{
					desired: 1,
					ready:   1,
					current: 1,
					updated: 1,
				})
				statefulset.Labels = map[string]string{
					"foo": "bar",
				}

				ssList := &appsv1.StatefulSetList{
					Items: []appsv1.StatefulSet{statefulset},
				}
				fakeLister.ListReturns(ssList, nil)
			})

			It("should time out", func() {
				Expect(waiter.Wait([]string{})).To(MatchError("timed out after 1s"))
			})
		})

		Context("value is different", func() {
			BeforeEach(func() {
				waiter.ExpectedPodLabels = map[string]string{
					"foo": "bar",
					"baz": "bat",
				}

				statefulset := createStatefulSet("name", "version", replicaStatuses{
					desired: 1,
					ready:   1,
					current: 1,
					updated: 1,
				})
				statefulset.Labels = map[string]string{
					"foo": "bar",
				}

				ssList := &appsv1.StatefulSetList{
					Items: []appsv1.StatefulSet{statefulset},
				}
				fakeLister.ListReturns(ssList, nil)
			})

			It("should time out", func() {
				Expect(waiter.Wait([]string{})).To(MatchError("timed out after 1s"))
			})
		})
	})

	When("the specified timeout is not valid", func() {
		It("should return an error", func() {
			waiter.Timeout = -1
			err := waiter.Wait([]string{})
			Expect(err).To(MatchError("provided timeout is not valid"))
		})
	})

	When("listing StatefulSets fails", func() {
		It("should log the error", func() {
			fakeLister.ListReturns(nil, errors.New("boom?"))

			err := waiter.Wait([]string{})
			Expect(err).To(HaveOccurred())
			Expect(logger.Logs()).To(HaveLen(1))
			log := logger.Logs()[0]
			Expect(log.Message).To(Equal("test.failed to list statefulsets"))
			Expect(log.Data).To(HaveKeyWithValue("error", "boom?"))
		})
	})

	When("listing StatefulSets", func() {
		It("should use the right label selector", func() {
			ssList := &appsv1.StatefulSetList{
				Items: []appsv1.StatefulSet{createStatefulSet("name", "version", replicaStatuses{
					desired: 1,
					ready:   1,
					current: 1,
					updated: 1,
				})},
			}
			fakeLister.ListReturns(ssList, nil)

			_ = waiter.Wait([]string{})
			listOptions := fakeLister.ListArgsForCall(0)
			Expect(listOptions.LabelSelector).To(Equal("my=label"))
		})
	})
})

type replicaStatuses struct {
	desired, ready, current, updated int32
}

func createStatefulSet(name, version string, replicaStatuses replicaStatuses) appsv1.StatefulSet {
	return appsv1.StatefulSet{
		ObjectMeta: metav1.ObjectMeta{
			Name:        name,
			Labels:      map[string]string{RootfsVersionLabel: version},
			Annotations: map[string]string{cf.ProcessGUID: name},
		},
		Spec: appsv1.StatefulSetSpec{
			Replicas: &replicaStatuses.desired,
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{RootfsVersionLabel: version},
				},
			},
		},
		Status: appsv1.StatefulSetStatus{
			ReadyReplicas:   replicaStatuses.ready,
			UpdatedReplicas: replicaStatuses.updated,
			CurrentReplicas: replicaStatuses.current,
		},
	}
}

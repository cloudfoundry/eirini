package waiter_test

import (
	"time"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/pkg/errors"
	appsv1 "k8s.io/api/apps/v1"

	. "code.cloudfoundry.org/eirini/waiter"
	"code.cloudfoundry.org/eirini/waiter/waiterfakes"
	"code.cloudfoundry.org/lager/lagertest"
)

var _ = Describe("DeploymentWaiter", func() {

	var (
		fakeLister *waiterfakes.FakeDeploymentLister
		waiter     Deployment
		logger     *lagertest.TestLogger
	)

	BeforeEach(func() {
		fakeLister = new(waiterfakes.FakeDeploymentLister)
		logger = lagertest.NewTestLogger("test")
		waiter = Deployment{
			Deployments:       fakeLister,
			Timeout:           100 * time.Millisecond,
			Logger:            logger,
			ListLabelSelector: "my=label",
		}
	})

	It("should wait for all Deployments to be updated and ready", func() {
		ssList := &appsv1.DeploymentList{
			Items: []appsv1.Deployment{createDeployment(replicaStatuses{
				desired:   1,
				ready:     1,
				available: 1,
				updated:   1,
			})},
		}
		fakeLister.ListReturns(ssList, nil)

		Expect(waiter.Wait()).To(Succeed())
	})

	It("should wait until Deployments status is tracking the updated pods", func() {
		outdatedSS := createDeployment(replicaStatuses{
			desired:   1,
			ready:     1,
			available: 1,
			updated:   1,
		})
		outdatedSS.Generation = 3
		outdatedSS.Status.ObservedGeneration = 2

		updatedSS := createDeployment(replicaStatuses{
			desired:   1,
			ready:     1,
			available: 1,
			updated:   1,
		})
		updatedSS.Generation = 3
		updatedSS.Status.ObservedGeneration = 3

		fakeLister.ListReturnsOnCall(0, &appsv1.DeploymentList{Items: []appsv1.Deployment{outdatedSS}}, nil)
		fakeLister.ListReturnsOnCall(1, &appsv1.DeploymentList{Items: []appsv1.Deployment{updatedSS}}, nil)

		waiter.Timeout = 2 * time.Second
		Expect(waiter.Wait()).To(Succeed())
		Expect(fakeLister.ListCallCount()).To(Equal(2))
	})

	When("Deployments don't become ready before timeout", func() {
		When("there are non-ready replicas", func() {
			BeforeEach(func() {
				ssList := &appsv1.DeploymentList{
					Items: []appsv1.Deployment{createDeployment(replicaStatuses{
						desired: 3,
						ready:   1,
					})},
				}
				fakeLister.ListReturns(ssList, nil)

			})

			It("should return error", func() {
				Expect(waiter.Wait()).To(MatchError(ContainSubstring("timed out after")))
			})
		})

		When("there are non-updated replicas", func() {
			BeforeEach(func() {
				ssList := &appsv1.DeploymentList{
					Items: []appsv1.Deployment{createDeployment(replicaStatuses{
						desired: 3,
						ready:   3,
						updated: 2,
					})},
				}
				fakeLister.ListReturns(ssList, nil)

			})

			It("should return error", func() {
				Expect(waiter.Wait()).To(MatchError(ContainSubstring("timed out after")))
			})
		})

		When("current replicas are less than desired", func() {
			BeforeEach(func() {
				ssList := &appsv1.DeploymentList{
					Items: []appsv1.Deployment{createDeployment(replicaStatuses{
						desired:   3,
						ready:     3,
						updated:   3,
						available: 1,
					})},
				}
				fakeLister.ListReturns(ssList, nil)

			})

			It("should return error", func() {
				Expect(waiter.Wait()).To(MatchError(ContainSubstring("timed out after")))
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

				deployment := createDeployment(replicaStatuses{
					desired:   1,
					ready:     1,
					available: 1,
					updated:   1,
				})
				deployment.Labels = map[string]string{
					"foo": "bar",
				}

				ssList := &appsv1.DeploymentList{
					Items: []appsv1.Deployment{deployment},
				}
				fakeLister.ListReturns(ssList, nil)
			})

			It("should time out", func() {
				Expect(waiter.Wait()).To(MatchError(ContainSubstring("timed out after")))
			})
		})

		Context("value is different", func() {
			BeforeEach(func() {
				waiter.ExpectedPodLabels = map[string]string{
					"foo": "bar",
					"baz": "bat",
				}

				deployment := createDeployment(replicaStatuses{
					desired:   1,
					ready:     1,
					available: 1,
					updated:   1,
				})
				deployment.Labels = map[string]string{
					"foo": "bar",
				}

				ssList := &appsv1.DeploymentList{
					Items: []appsv1.Deployment{deployment},
				}
				fakeLister.ListReturns(ssList, nil)
			})

			It("should time out", func() {
				Expect(waiter.Wait()).To(MatchError(ContainSubstring("timed out after")))
			})
		})
	})

	When("the specified timeout is not valid", func() {
		It("should return an error", func() {
			waiter.Timeout = -1
			Expect(waiter.Wait()).To(MatchError("provided timeout is not valid"))
		})
	})

	When("listing Deployments fails", func() {
		It("should log the error", func() {
			fakeLister.ListReturns(nil, errors.New("boom?"))

			Expect(waiter.Wait()).NotTo(Succeed())
			Expect(logger.Logs()).To(HaveLen(1))
			log := logger.Logs()[0]
			Expect(log.Message).To(Equal("test.failed to list deployments"))
			Expect(log.Data).To(HaveKeyWithValue("error", "boom?"))
		})
	})

	When("listing Deployments", func() {
		It("should use the right label selector", func() {
			ssList := &appsv1.DeploymentList{
				Items: []appsv1.Deployment{createDeployment(replicaStatuses{
					desired:   1,
					ready:     1,
					available: 1,
					updated:   1,
				})},
			}
			fakeLister.ListReturns(ssList, nil)

			_ = waiter.Wait()
			listOptions := fakeLister.ListArgsForCall(0)
			Expect(listOptions.LabelSelector).To(Equal("my=label"))
		})
	})
})

type replicaStatuses struct {
	desired, ready, available, updated int32
}

func createDeployment(replicaStatuses replicaStatuses) appsv1.Deployment {
	return appsv1.Deployment{
		Spec: appsv1.DeploymentSpec{
			Replicas: &replicaStatuses.desired,
		},
		Status: appsv1.DeploymentStatus{
			ReadyReplicas:     replicaStatuses.ready,
			UpdatedReplicas:   replicaStatuses.updated,
			AvailableReplicas: replicaStatuses.available,
		},
	}
}

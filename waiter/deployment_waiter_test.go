package waiter_test

import (
	"errors"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
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
			Logger:            logger,
			ListLabelSelector: "my=label",
		}
	})

	It("should wait for all Deployments to be updated and ready", func() {
		ssList := &appsv1.DeploymentList{
			Items: []appsv1.Deployment{createDeployment(replicaStatuses{
				desired:     1,
				ready:       1,
				updated:     1,
				available:   1,
				unavailable: 0,
			})},
		}
		fakeLister.ListReturns(ssList, nil)

		ready := make(chan interface{}, 1)
		stop := make(chan interface{}, 1)

		defer close(ready)
		defer close(stop)

		waiter.Wait(ready, stop)
		Expect(ready).To(Receive())
	})

	It("should wait until Deployments status is tracking the updated pods", func() {
		outdatedSS := createDeployment(replicaStatuses{
			desired:     1,
			ready:       1,
			updated:     1,
			available:   1,
			unavailable: 0,
		})
		outdatedSS.Generation = 3
		outdatedSS.Status.ObservedGeneration = 2

		updatedSS := createDeployment(replicaStatuses{
			desired:     1,
			ready:       1,
			updated:     1,
			available:   1,
			unavailable: 0,
		})
		updatedSS.Generation = 3
		updatedSS.Status.ObservedGeneration = 3

		fakeLister.ListReturnsOnCall(0, &appsv1.DeploymentList{Items: []appsv1.Deployment{outdatedSS}}, nil)
		fakeLister.ListReturnsOnCall(1, &appsv1.DeploymentList{Items: []appsv1.Deployment{updatedSS}}, nil)

		ready := make(chan interface{}, 1)
		stop := make(chan interface{}, 1)
		defer close(ready)
		defer close(stop)

		waiter.Wait(ready, stop)
		Expect(fakeLister.ListCallCount()).To(Equal(2))
		Expect(ready).To(Receive())
	})

	When("Stop is signalled", func() {
		testItStops := func() {
			ready := make(chan interface{}, 1)
			stop := make(chan interface{}, 1)
			defer close(ready)
			defer close(stop)

			exited := make(chan interface{}, 1)
			defer close(exited)
			go func() {
				waiter.Wait(ready, stop)
				exited <- nil
			}()

			stop <- nil
			Eventually(exited).Should(Receive())
			Expect(ready).ToNot(Receive())
		}

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

			It("should stop executing without writing to ready chan", testItStops)
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

			It("should stop executing without writing to ready chan", testItStops)
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

			It("should stop executing without writing to ready chan", testItStops)
		})

		When("there remain unavailable replicas", func() {
			BeforeEach(func() {
				ssList := &appsv1.DeploymentList{
					Items: []appsv1.Deployment{createDeployment(replicaStatuses{
						desired:     3,
						ready:       3,
						updated:     3,
						available:   3,
						unavailable: 1,
					})},
				}
				fakeLister.ListReturns(ssList, nil)
			})

			It("should stop executing without writing to ready chan", testItStops)
		})
	})

	When("listing Deployments fails", func() {
		It("should log the error", func() {
			fakeLister.ListReturns(nil, errors.New("boom?"))

			ready := make(chan interface{}, 1)
			stop := make(chan interface{}, 1)
			defer close(ready)
			defer close(stop)

			go func() {
				waiter.Wait(ready, stop)
			}()

			Eventually(logger.Logs).Should(HaveLen(1))
			stop <- nil
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
			ready := make(chan interface{}, 1)
			stop := make(chan interface{}, 1)
			defer close(ready)
			defer close(stop)

			go func() {
				waiter.Wait(ready, stop)
			}()

			Eventually(ready).Should(Receive())
			listOptions := fakeLister.ListArgsForCall(0)
			Expect(listOptions.LabelSelector).To(Equal("my=label"))
		})
	})
})

type replicaStatuses struct {
	desired, ready, available, updated, unavailable int32
}

func createDeployment(replicaStatuses replicaStatuses) appsv1.Deployment {
	return appsv1.Deployment{
		Spec: appsv1.DeploymentSpec{
			Replicas: &replicaStatuses.desired,
		},
		Status: appsv1.DeploymentStatus{
			ReadyReplicas:       replicaStatuses.ready,
			UpdatedReplicas:     replicaStatuses.updated,
			AvailableReplicas:   replicaStatuses.available,
			UnavailableReplicas: replicaStatuses.unavailable,
		},
	}
}

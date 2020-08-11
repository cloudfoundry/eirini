package utils_test

import (
	"errors"

	. "code.cloudfoundry.org/eirini/k8s/utils"
	"code.cloudfoundry.org/eirini/k8s/utils/utilsfakes"
	"code.cloudfoundry.org/lager/lagertest"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	appsv1 "k8s.io/api/apps/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

var _ = Describe("Deployment Utils", func() {
	var (
		fakeclient *utilsfakes.FakeDeploymentClient
		logger     *lagertest.TestLogger
	)

	BeforeEach(func() {
		fakeclient = new(utilsfakes.FakeDeploymentClient)
		logger = lagertest.NewTestLogger("test")
	})

	It("reports ready if the Deployment is updated and ready", func() {
		deployment := createDeployment(replicaStatuses{
			desired:     1,
			ready:       1,
			updated:     1,
			available:   1,
			unavailable: 0,
		})
		fakeclient.GetReturns(&deployment, nil)

		Expect(IsReady(fakeclient, logger, "name")).To(BeTrue())
	})

	It("reports not ready if observed generation is not updated yet", func() {
		outdatedDeployment := createDeployment(replicaStatuses{
			desired:     1,
			ready:       1,
			updated:     1,
			available:   1,
			unavailable: 0,
		})
		outdatedDeployment.Generation = 3
		outdatedDeployment.Status.ObservedGeneration = 2

		fakeclient.GetReturns(&outdatedDeployment, nil)

		Expect(IsReady(fakeclient, logger, "name")).To(BeFalse())
	})

	It("reports not ready if there are non-ready replicas", func() {
		deployment := createDeployment(replicaStatuses{
			desired: 3,
			ready:   1,
		})
		fakeclient.GetReturns(&deployment, nil)

		Expect(IsReady(fakeclient, logger, "name")).To(BeFalse())
	})

	It("reports not ready if there are non-updated replicas", func() {
		deployment := createDeployment(replicaStatuses{
			desired: 3,
			ready:   3,
			updated: 2,
		})
		fakeclient.GetReturns(&deployment, nil)

		Expect(IsReady(fakeclient, logger, "name")).To(BeFalse())
	})

	It("reports not ready if current replicas are less than desired", func() {
		deployment := createDeployment(replicaStatuses{
			desired:   3,
			ready:     3,
			updated:   3,
			available: 1,
		})
		fakeclient.GetReturns(&deployment, nil)

		Expect(IsReady(fakeclient, logger, "name")).To(BeFalse())
	})

	It("reports not ready is there remain unavailable replicas", func() {
		deployment := createDeployment(replicaStatuses{
			desired:     3,
			ready:       3,
			updated:     3,
			available:   3,
			unavailable: 1,
		})
		fakeclient.GetReturns(&deployment, nil)
		Expect(IsReady(fakeclient, logger, "name")).To(BeFalse())
	})

	When("listing Deployments fails", func() {
		It("should log the error", func() {
			fakeclient.GetReturns(nil, errors.New("boom?"))
			IsReady(fakeclient, logger, "name")

			Eventually(logger.Logs, "2s").Should(HaveLen(1))
			log := logger.Logs()[0]
			Expect(log.Message).To(Equal("test.failed to list deployments"))
			Expect(log.Data).To(HaveKeyWithValue("error", "boom?"))
		})

		It("should return false", func() {
			fakeclient.GetReturns(nil, errors.New("boom?"))
			Expect(IsReady(fakeclient, logger, "name")).To(BeFalse())
		})
	})

	When("listing Deployments", func() {
		It("should use the right label selector", func() {
			deployment := createDeployment(replicaStatuses{
				desired:   1,
				ready:     1,
				available: 1,
				updated:   1,
			})
			fakeclient.GetReturns(&deployment, nil)
			IsReady(fakeclient, logger, "name")

			Expect(fakeclient.GetCallCount()).To(Equal(1))
			_, name, getOptions := fakeclient.GetArgsForCall(0)
			Expect(name).To(Equal("name"))
			Expect(getOptions).To(Equal(metav1.GetOptions{}))
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

package k8s_test

import (
	"code.cloudfoundry.org/eirini/models/cf"
	"code.cloudfoundry.org/eirini/opi"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"k8s.io/api/apps/v1beta1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/kubernetes/fake"

	. "code.cloudfoundry.org/eirini/k8s"
)

var _ = Describe("Deployment", func() {

	var (
		fakeClient        kubernetes.Interface
		deploymentManager DeploymentManager
		lrps              []opi.LRP
	)

	const (
		namespace = "midgard"
	)

	BeforeEach(func() {
		lrps = []opi.LRP{
			createLRP("odin", "1234.5"),
			createLRP("thor", "4567.8"),
			createLRP("mimir", "9012.3"),
		}
	})

	JustBeforeEach(func() {
		fakeClient = fake.NewSimpleClientset()
		deploymentManager = NewDeploymentManager(fakeClient)
		for _, l := range lrps {
			fakeClient.AppsV1beta1().Deployments(namespace).Create(toDeployment(l))
		}
	})

	Context("List deployments", func() {
		Context("When listing deployments", func() {
			It("translates all existing deployments to opi.LRPs", func() {
				actualLRPs, err := deploymentManager.ListLRPs(namespace)
				Expect(err).ToNot(HaveOccurred())
				Expect(actualLRPs).To(Equal(lrps))
			})
		})

		Context("When no deployments exist", func() {

			BeforeEach(func() {
				lrps = []opi.LRP{}
			})

			It("returns an empy list of LRPs", func() {
				actualLRPs, err := deploymentManager.ListLRPs(namespace)
				Expect(err).ToNot(HaveOccurred())
				Expect(actualLRPs).To(BeEmpty())
			})
		})
	})
})

func toDeployment(lrp opi.LRP) *v1beta1.Deployment {
	deployment := &v1beta1.Deployment{}
	deployment.Name = "test-app-" + lrp.Metadata[cf.ProcessGuid]
	deployment.Annotations = map[string]string{
		cf.ProcessGuid: lrp.Metadata[cf.ProcessGuid],
		cf.LastUpdated: lrp.Metadata[cf.LastUpdated],
	}
	return deployment
}

func createLRP(processGuid, lastUpdated string) opi.LRP {
	return opi.LRP{
		Metadata: map[string]string{
			cf.ProcessGuid: processGuid,
			cf.LastUpdated: lastUpdated,
		},
	}
}

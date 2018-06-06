package k8s_test

import (
	"code.cloudfoundry.org/eirini/opi"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"k8s.io/api/apps/v1beta1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/kubernetes/fake"

	. "code.cloudfoundry.org/eirini/k8s"
)

var _ = FDescribe("Deployment", func() {

	var (
		fakeClient        kubernetes.Interface
		deploymentManager DeploymentManager
	)

	const (
		namespace = "midgard"
	)

	BeforeEach(func() {
		fakeClient = fake.NewSimpleClientset()
	})

	JustBeforeEach(func() {
		deploymentManager = NewDeploymentManager(fakeClient)
	})

	Context("List deployments", func() {
		var (
			apps []string
		)

		Context("When listing deployments", func() {

			JustBeforeEach(func() {
				apps = []string{"odin", "thor", "mimir"}
				for _, a := range apps {
					fakeClient.AppsV1beta1().Deployments(namespace).Create(toDeployment(a))
				}
			})

			It("translates all existing deployments to opi.LRPs", func() {
				lrps, err := deploymentManager.ListLRPs(namespace)
				Expect(err).ToNot(HaveOccurred())
				exists := findAll(lrps, apps)
				Expect(exists).To(BeTrue())
			})
		})
	})
})

func toDeployment(name string) *v1beta1.Deployment {
	deployment := &v1beta1.Deployment{}
	deployment.Name = name
	return deployment
}

func findAll(lrps []opi.LRP, ids []string) bool {
	var exists bool
	for _, a := range ids {
		exists = false
		for _, l := range lrps {
			if l.Name == a {
				exists = true
			}
		}
		if !exists {
			break
		}
	}
	return exists
}

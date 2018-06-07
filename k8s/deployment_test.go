package k8s_test

import (
	"code.cloudfoundry.org/eirini/opi"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"k8s.io/api/apps/v1beta1"
	"k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/kubernetes/fake"

	. "code.cloudfoundry.org/eirini/k8s"
)

var _ = Describe("Deployment", func() {

	var (
		fakeClient        kubernetes.Interface
		deploymentManager DeploymentManager
		apps              []string
	)

	const (
		namespace = "midgard"
	)

	BeforeEach(func() {
		apps = []string{"odin", "thor", "mimir"}
	})

	JustBeforeEach(func() {
		fakeClient = fake.NewSimpleClientset()
		deploymentManager = NewDeploymentManager(fakeClient)
		for _, a := range apps {
			fakeClient.AppsV1beta1().Deployments(namespace).Create(toDeployment(a))
		}
	})

	AfterEach(func() {
		for _, a := range apps {
			fakeClient.AppsV1beta1().Deployments(namespace).Delete(a, &v1.DeleteOptions{})
		}
	})

	Context("List deployments", func() {
		Context("When listing deployments", func() {
			It("translates all existing deployments to opi.LRPs", func() {
				lrps, err := deploymentManager.ListLRPs(namespace)
				Expect(err).ToNot(HaveOccurred())
				Expect(len(lrps)).To(Equal(3))

				exists := findAll(lrps, apps)
				Expect(exists).To(BeTrue())
			})
		})

		Context("When no deployments exist", func() {

			BeforeEach(func() {
				apps = []string{}
			})

			It("returns an empy list of LRPs", func() {
				lrps, err := deploymentManager.ListLRPs(namespace)
				Expect(err).ToNot(HaveOccurred())
				Expect(len(lrps)).To(Equal(0))
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

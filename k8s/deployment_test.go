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
		processGuids      []string
	)

	const (
		namespace = "midgard"
	)

	BeforeEach(func() {
		processGuids = []string{"odin", "thor", "mimir"}
	})

	JustBeforeEach(func() {
		fakeClient = fake.NewSimpleClientset()
		deploymentManager = NewDeploymentManager(fakeClient)
		for _, a := range processGuids {
			fakeClient.AppsV1beta1().Deployments(namespace).Create(toDeployment(a))
		}
	})

	AfterEach(func() {
		for _, a := range processGuids {
			fakeClient.AppsV1beta1().Deployments(namespace).Delete(a, &v1.DeleteOptions{})
		}
	})

	Context("List deployments", func() {
		Context("When listing deployments", func() {
			It("translates all existing deployments to opi.LRPs", func() {
				lrps, err := deploymentManager.ListLRPs(namespace)
				Expect(err).ToNot(HaveOccurred())
				Expect(len(lrps)).To(Equal(3))

				exists := findAll(lrps, processGuids)
				Expect(exists).To(BeTrue())
			})
		})

		Context("When no deployments exist", func() {

			BeforeEach(func() {
				processGuids = []string{}
			})

			It("returns an empy list of LRPs", func() {
				lrps, err := deploymentManager.ListLRPs(namespace)
				Expect(err).ToNot(HaveOccurred())
				Expect(len(lrps)).To(Equal(0))
			})
		})
	})
})

func toDeployment(processGuid string) *v1beta1.Deployment {
	deployment := &v1beta1.Deployment{}
	deployment.Name = "test-app-" + processGuid
	deployment.Annotations = map[string]string{
		"process_guid": processGuid,
	}
	return deployment
}

func findAll(lrps []opi.LRP, ids []string) bool {
	var exists bool
	for _, a := range ids {
		exists = false
		for _, l := range lrps {
			if l.Metadata["process_guid"] == a {
				exists = true
			}
		}
		if !exists {
			break
		}
	}
	return exists
}

package eats_test

import (
	"context"

	"code.cloudfoundry.org/eirini"
	"code.cloudfoundry.org/eirini/integration/util"
	"code.cloudfoundry.org/eirini/k8s"
	eiriniv1 "code.cloudfoundry.org/eirini/pkg/apis/eirini/v1"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

var _ = Describe("InstanceIndexEnvInjector", func() {
	const lrpName = "lrp-name-irrelevant"

	var (
		namespace  string
		lrpGUID    string
		lrpVersion string
	)

	getStatefulSetPods := func() []corev1.Pod {
		podList, err := fixture.Clientset.
			CoreV1().
			Pods(fixture.Namespace).
			List(context.Background(), metav1.ListOptions{})

		Expect(err).NotTo(HaveOccurred())
		if len(podList.Items) == 0 {
			return nil
		}

		return podList.Items
	}

	getCFInstanceIndex := func(pod corev1.Pod) string {
		for _, container := range pod.Spec.Containers {
			if container.Name != k8s.OPIContainerName {
				continue
			}

			for _, e := range container.Env {
				if e.Name != eirini.EnvCFInstanceIndex {
					continue
				}

				return e.Value
			}
		}

		return ""
	}

	BeforeEach(func() {
		namespace = fixture.Namespace
		lrpGUID = util.GenerateGUID()
		lrpVersion = util.GenerateGUID()

		lrp := &eiriniv1.LRP{
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
				Instances:              3,
				LastUpdated:            "a long time ago in a galaxy far, far away",
				Ports:                  []int32{8080},
				VolumeMounts:           []eiriniv1.VolumeMount{},
				UserDefinedAnnotations: map[string]string{},
				AppRoutes:              []eiriniv1.Route{{Hostname: "app-hostname-1", Port: 8080}},
			},
		}

		_, err := fixture.EiriniClientset.
			EiriniV1().
			LRPs(namespace).
			Create(context.Background(), lrp, metav1.CreateOptions{})
		Expect(err).NotTo(HaveOccurred())
	})

	It("creates pods with CF_INSTANCE_INDEX set to 0, 1 and 2", func() {
		Eventually(getStatefulSetPods, "30s").Should(HaveLen(3))

		envVars := []string{}
		for _, pod := range getStatefulSetPods() {
			envVars = append(envVars, getCFInstanceIndex(pod))
		}

		Expect(envVars).To(ConsistOf([]string{"0", "1", "2"}))
	})
})

package statefulsets_test

import (
	"fmt"
	"math/rand"
	"os"
	"testing"
	"time"

	"code.cloudfoundry.org/eirini/opi"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	policyv1 "k8s.io/api/policy/v1beta1"
	rbacv1 "k8s.io/api/rbac/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	types "k8s.io/client-go/kubernetes/typed/apps/v1"
	_ "k8s.io/client-go/plugin/pkg/client/auth"
	"k8s.io/client-go/tools/clientcmd"
)

var (
	namespace      string
	clientset      kubernetes.Interface
	kubeConfigPath string
)

const (
	timeout time.Duration = 60 * time.Second
)

var _ = BeforeSuite(func() {
	kubeConfigPath = os.Getenv("INTEGRATION_KUBECONFIG")
	config, err := clientcmd.BuildConfigFromFlags("", os.Getenv("INTEGRATION_KUBECONFIG"))
	Expect(err).ToNot(HaveOccurred())

	clientset, err = kubernetes.NewForConfig(config)
	Expect(err).ToNot(HaveOccurred())

	namespace = fmt.Sprintf("opi-integration-test-%d", rand.Intn(100000000))

	for namespaceExists(namespace) {
		namespace = fmt.Sprintf("opi-integration-test-%d", rand.Intn(100000000))
	}

	createNamespace(namespace)
	allowPodCreation(namespace)
})

var _ = AfterSuite(func() {
	err := clientset.CoreV1().Namespaces().Delete(namespace, &metav1.DeleteOptions{})
	Expect(err).ToNot(HaveOccurred())

	pspName := fmt.Sprintf("%s-psp", namespace)
	err = clientset.PolicyV1beta1().PodSecurityPolicies().Delete(pspName, &metav1.DeleteOptions{})
	Expect(err).ToNot(HaveOccurred())
})

func TestIntegration(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Integration Suite")
}

func namespaceExists(namespace string) bool {
	_, err := clientset.CoreV1().Namespaces().Get(namespace, metav1.GetOptions{})
	return err == nil
}

func createNamespace(namespace string) {
	namespaceSpec := &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: namespace}}

	if _, err := clientset.CoreV1().Namespaces().Create(namespaceSpec); err != nil {
		panic(err)
	}
}

func allowPodCreation(namespace string) {
	pspName := fmt.Sprintf("%s-psp", namespace)
	roleName := "use-psp"

	_, err := clientset.PolicyV1beta1().PodSecurityPolicies().Create(&policyv1.PodSecurityPolicy{
		ObjectMeta: metav1.ObjectMeta{
			Name: pspName,
			Annotations: map[string]string{
				"seccomp.security.alpha.kubernetes.io/allowedProfileNames": "runtime/default",
				"seccomp.security.alpha.kubernetes.io/defaultProfileName":  "runtime/default",
			},
		},
		Spec: policyv1.PodSecurityPolicySpec{
			Privileged: false,
			RunAsUser: policyv1.RunAsUserStrategyOptions{
				Rule: policyv1.RunAsUserStrategyRunAsAny,
			},
			SELinux: policyv1.SELinuxStrategyOptions{
				Rule: policyv1.SELinuxStrategyRunAsAny,
			},
			SupplementalGroups: policyv1.SupplementalGroupsStrategyOptions{
				Rule: policyv1.SupplementalGroupsStrategyMustRunAs,
				Ranges: []policyv1.IDRange{{
					Min: 1,
					Max: 65535,
				}},
			},
			FSGroup: policyv1.FSGroupStrategyOptions{
				Rule: policyv1.FSGroupStrategyMustRunAs,
				Ranges: []policyv1.IDRange{{
					Min: 1,
					Max: 65535,
				}},
			},
		},
	})
	if err != nil {
		panic(err)
	}

	_, err = clientset.RbacV1().Roles(namespace).Create(&rbacv1.Role{
		ObjectMeta: metav1.ObjectMeta{
			Name:      roleName,
			Namespace: namespace,
		},
		Rules: []rbacv1.PolicyRule{
			{
				APIGroups:     []string{"policy"},
				Resources:     []string{"podsecuritypolicies"},
				ResourceNames: []string{pspName},
				Verbs:         []string{"use"},
			},
		},
	})
	if err != nil {
		panic(err)
	}

	_, err = clientset.RbacV1().RoleBindings(namespace).Create(&rbacv1.RoleBinding{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "default-account-psp",
			Namespace: namespace,
		},
		RoleRef: rbacv1.RoleRef{
			APIGroup: "rbac.authorization.k8s.io",
			Kind:     "Role",
			Name:     roleName,
		},
		Subjects: []rbacv1.Subject{{
			Kind:      rbacv1.ServiceAccountKind,
			Name:      "default",
			Namespace: namespace,
		}},
	})
	if err != nil {
		panic(err)
	}
}

func statefulSets() types.StatefulSetInterface {
	return clientset.AppsV1().StatefulSets(namespace)
}

func getStatefulSet(lrp *opi.LRP) *appsv1.StatefulSet {
	ss, getErr := statefulSets().List(metav1.ListOptions{LabelSelector: labelSelector(lrp)})
	Expect(getErr).NotTo(HaveOccurred())
	return &ss.Items[0]
}

func labelSelector(lrp *opi.LRP) string {
	return fmt.Sprintf("guid=%s,version=%s", lrp.LRPIdentifier.GUID, lrp.LRPIdentifier.Version)
}

func cleanupStatefulSet(lrp *opi.LRP) {
	backgroundPropagation := metav1.DeletePropagationBackground
	deleteOptions := &metav1.DeleteOptions{PropagationPolicy: &backgroundPropagation}
	listOptions := metav1.ListOptions{LabelSelector: labelSelector(lrp)}
	err := statefulSets().DeleteCollection(deleteOptions, listOptions)
	Expect(err).ToNot(HaveOccurred())
}

func listAllStatefulSets(lrp1, lrp2 *opi.LRP) []appsv1.StatefulSet {
	labels := fmt.Sprintf("guid in (%s, %s),version in (%s, %s)",
		lrp1.LRPIdentifier.GUID,
		lrp2.LRPIdentifier.GUID,
		lrp1.LRPIdentifier.Version,
		lrp2.LRPIdentifier.Version,
	)

	list, err := statefulSets().List(metav1.ListOptions{LabelSelector: labels})
	Expect(err).NotTo(HaveOccurred())
	return list.Items
}

func listStatefulSets(appName string) []appsv1.StatefulSet {
	labelSelector := fmt.Sprintf("name=%s", appName)
	list, err := statefulSets().List(metav1.ListOptions{LabelSelector: labelSelector})
	Expect(err).NotTo(HaveOccurred())
	return list.Items
}

func listPodsByLabel(labelSelector string) []corev1.Pod {
	pods, err := clientset.CoreV1().Pods(namespace).List(metav1.ListOptions{LabelSelector: labelSelector})
	Expect(err).NotTo(HaveOccurred())
	return pods.Items
}

func listPods(lrpIdentifier opi.LRPIdentifier) []corev1.Pod {
	labelSelector := fmt.Sprintf("guid=%s,version=%s", lrpIdentifier.GUID, lrpIdentifier.Version)
	return listPodsByLabel(labelSelector)
}

func podNamesFromPods(pods []corev1.Pod) []string {
	names := []string{}
	for _, p := range pods {
		names = append(names, p.Name)
	}
	return names
}

func podCrashed(pod corev1.Pod) bool {
	if len(pod.Status.ContainerStatuses) == 0 {
		return false
	}
	terminated := pod.Status.ContainerStatuses[0].State.Terminated
	waiting := pod.Status.ContainerStatuses[0].State.Waiting
	return terminated != nil || waiting != nil && waiting.Reason == "CrashLoopBackOff"
}

func podReady(pod corev1.Pod) bool {
	for _, c := range pod.Status.Conditions {
		if c.Type == corev1.PodReady {
			return c.Status == corev1.ConditionTrue
		}
	}
	return false
}

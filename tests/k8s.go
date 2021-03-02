package tests

import (
	"context"
	"fmt"
	"math/rand"
	"os"

	ginkgoconfig "github.com/onsi/ginkgo/config"

	//nolint:revive
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	policyv1 "k8s.io/api/policy/v1beta1"
	rbacv1 "k8s.io/api/rbac/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

const (
	randUpperBound                   = 100000000
	DefaultApplicationServiceAccount = "eirini"
)

func CreateRandomNamespace(clientset kubernetes.Interface) string {
	namespace := fmt.Sprintf("opi-integration-test-%d-%d", rand.Intn(randUpperBound), ginkgoconfig.GinkgoConfig.ParallelNode)
	for namespaceExists(namespace, clientset) {
		namespace = fmt.Sprintf("opi-integration-test-%d-%d", rand.Intn(randUpperBound), ginkgoconfig.GinkgoConfig.ParallelNode)
	}
	createNamespace(namespace, clientset)

	return namespace
}

func namespaceExists(namespace string, clientset kubernetes.Interface) bool {
	_, err := clientset.CoreV1().Namespaces().Get(context.Background(), namespace, metav1.GetOptions{})

	return err == nil
}

func createNamespace(namespace string, clientset kubernetes.Interface) {
	namespaceSpec := &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: namespace}}

	_, err := clientset.CoreV1().Namespaces().Create(context.Background(), namespaceSpec, metav1.CreateOptions{})
	Expect(err).NotTo(HaveOccurred())
}

func CreatePodCreationPSP(namespace, pspName, serviceAccountName string, clientset kubernetes.Interface) error {
	_, err := clientset.PolicyV1beta1().PodSecurityPolicies().Create(context.Background(), &policyv1.PodSecurityPolicy{
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
				Rule: policyv1.SupplementalGroupsStrategyRunAsAny,
			},
			FSGroup: policyv1.FSGroupStrategyOptions{
				Rule: policyv1.FSGroupStrategyRunAsAny,
			},
			Volumes: []policyv1.FSType{
				policyv1.EmptyDir, policyv1.Projected, policyv1.Secret,
			},
		},
	}, metav1.CreateOptions{})
	if err != nil {
		return err
	}

	roleName := "use-psp"
	_, err = clientset.RbacV1().Roles(namespace).Create(context.Background(), &rbacv1.Role{
		ObjectMeta: metav1.ObjectMeta{
			Name:      roleName,
			Namespace: namespace,
		},
		Rules: []rbacv1.PolicyRule{
			{
				APIGroups:     []string{"policy"},
				Resources:     []string{"podsecuritypolicies"},
				Verbs:         []string{"use"},
				ResourceNames: []string{pspName},
			},
		},
	}, metav1.CreateOptions{})

	if err != nil {
		return err
	}

	_, err = clientset.CoreV1().ServiceAccounts(namespace).Create(context.Background(), &corev1.ServiceAccount{
		ObjectMeta: metav1.ObjectMeta{
			Name:      serviceAccountName,
			Namespace: namespace,
		},
	}, metav1.CreateOptions{})
	if err != nil {
		return err
	}

	_, err = clientset.RbacV1().RoleBindings(namespace).Create(context.Background(), &rbacv1.RoleBinding{
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
			Name:      serviceAccountName,
			Namespace: namespace,
		}},
	}, metav1.CreateOptions{})

	return err
}

func DeleteNamespace(namespace string, clientset kubernetes.Interface) error {
	return clientset.CoreV1().Namespaces().Delete(context.Background(), namespace, metav1.DeleteOptions{})
}

func DeletePSP(name string, clientset kubernetes.Interface) error {
	return clientset.PolicyV1beta1().PodSecurityPolicies().Delete(context.Background(), name, metav1.DeleteOptions{})
}

func GetApplicationServiceAccount() string {
	serviceAccountName := os.Getenv("APPLICATION_SERVICE_ACCOUNT")
	if serviceAccountName != "" {
		return serviceAccountName
	}

	return DefaultApplicationServiceAccount
}

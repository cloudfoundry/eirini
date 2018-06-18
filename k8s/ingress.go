package k8s

import (
	"fmt"

	"code.cloudfoundry.org/eirini"
	"code.cloudfoundry.org/eirini/models/cf"
	"code.cloudfoundry.org/eirini/opi"
	ext "k8s.io/api/extensions/v1beta1"
	av1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/client-go/kubernetes"
)

const (
	ingressName       = "eirini"
	ingressAPIVersion = "extensions/v1beta1"
	ingressKind       = "Ingress"
)

//go:generate counterfeiter . IngressManager
type IngressManager interface {
	UpdateIngress(namespace string, lrp opi.LRP) error
}

type KubeIngressManager struct {
	client   kubernetes.Interface
	endpoint string
}

func NewIngressManager(client kubernetes.Interface, kubeEndpoint string) IngressManager {
	return &KubeIngressManager{
		client:   client,
		endpoint: kubeEndpoint,
	}
}

func (i *KubeIngressManager) UpdateIngress(namespace string, lrp opi.LRP) error {
	ingresses, err := i.client.ExtensionsV1beta1().Ingresses(namespace).List(av1.ListOptions{})
	if err != nil {
		return err
	}

	if ingress, exists := i.getIngress(ingresses); exists {
		i.updateSpec(ingress, lrp)

		_, err = i.client.ExtensionsV1beta1().Ingresses(namespace).Update(ingress)
		return err
	} else {
		return i.createIngress(namespace, lrp)
	}
}

func (i *KubeIngressManager) createIngress(namespace string, lrp opi.LRP) error {
	ingress := &ext.Ingress{
		TypeMeta: av1.TypeMeta{
			Kind:       ingressKind,
			APIVersion: ingressAPIVersion,
		},
		ObjectMeta: av1.ObjectMeta{
			Name:      ingressName,
			Namespace: namespace,
		},
		Spec: ext.IngressSpec{
			TLS: []ext.IngressTLS{
				ext.IngressTLS{},
			},
		},
	}

	i.updateSpec(ingress, lrp)
	_, err := i.client.ExtensionsV1beta1().Ingresses(namespace).Create(ingress)
	return err
}

func (i *KubeIngressManager) updateSpec(ingress *ext.Ingress, lrp opi.LRP) {
	newHost := fmt.Sprintf("%s.%s", lrp.Metadata[cf.VcapAppName], i.endpoint)
	ingress.Spec.TLS[0].Hosts = append(ingress.Spec.TLS[0].Hosts, newHost)

	rule := createIngressRule(lrp, i.endpoint)
	ingress.Spec.Rules = append(ingress.Spec.Rules, rule)
}

func (i *KubeIngressManager) getIngress(ingresses *ext.IngressList) (*ext.Ingress, bool) {
	for _, ing := range ingresses.Items {
		if ing.ObjectMeta.Name == ingressName {
			return &ing, true
		}
	}
	return &ext.Ingress{}, false
}

func createIngressRule(lrp opi.LRP, kubeEndpoint string) ext.IngressRule {
	rule := ext.IngressRule{
		Host: fmt.Sprintf("%s.%s", lrp.Metadata[cf.VcapAppName], kubeEndpoint),
	}

	rule.HTTP = &ext.HTTPIngressRuleValue{
		Paths: []ext.HTTPIngressPath{
			ext.HTTPIngressPath{
				Path: "/",
				Backend: ext.IngressBackend{
					ServiceName: eirini.GetInternalServiceName(lrp.Name),
					ServicePort: intstr.FromInt(8080),
				},
			},
		},
	}

	return rule
}

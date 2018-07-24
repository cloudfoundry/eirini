package k8s

import (
	"encoding/json"

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
	client kubernetes.Interface
}

func NewIngressManager(client kubernetes.Interface) IngressManager {
	return &KubeIngressManager{
		client: client,
	}
}

func (i *KubeIngressManager) UpdateIngress(namespace string, lrp opi.LRP) error {
	uriList := []string{}
	err := json.Unmarshal([]byte(lrp.Metadata[cf.VcapAppUris]), &uriList)
	if err != nil {
		panic(err)
	}

	if len(uriList) == 0 {
		return nil
	}

	ingresses, err := i.client.ExtensionsV1beta1().Ingresses(namespace).List(av1.ListOptions{})
	if err != nil {
		return err
	}

	if ingress, exists := i.getIngress(ingresses); exists {
		i.updateSpec(ingress, lrp.Name, uriList)

		_, err = i.client.ExtensionsV1beta1().Ingresses(namespace).Update(ingress)
		return err
	}
	return i.createIngress(namespace, lrp.Name, uriList)
}

func (i *KubeIngressManager) createIngress(namespace, lrpName string, uriList []string) error {
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

	i.updateSpec(ingress, lrpName, uriList)
	_, err := i.client.ExtensionsV1beta1().Ingresses(namespace).Create(ingress)
	return err
}

func (i *KubeIngressManager) updateSpec(ingress *ext.Ingress, lrpName string, uriList []string) {
	ingress.Spec.TLS[0].Hosts = append(ingress.Spec.TLS[0].Hosts, uriList...)

	rules := createIngressRules(lrpName, uriList)
	ingress.Spec.Rules = append(ingress.Spec.Rules, rules...)
}

func (i *KubeIngressManager) getIngress(ingresses *ext.IngressList) (*ext.Ingress, bool) {
	for _, ing := range ingresses.Items {
		if ing.ObjectMeta.Name == ingressName {
			return &ing, true
		}
	}
	return &ext.Ingress{}, false
}

func createIngressRules(lrpName string, uriList []string) []ext.IngressRule {
	rules := []ext.IngressRule{}

	for _, uri := range uriList {
		rule := ext.IngressRule{
			Host: uri,
		}

		rule.HTTP = &ext.HTTPIngressRuleValue{
			Paths: []ext.HTTPIngressPath{
				ext.HTTPIngressPath{
					Path: "/",
					Backend: ext.IngressBackend{
						ServiceName: eirini.GetInternalServiceName(lrpName),
						ServicePort: intstr.FromInt(8080),
					},
				},
			},
		}
		rules = append(rules, rule)
	}

	return rules
}

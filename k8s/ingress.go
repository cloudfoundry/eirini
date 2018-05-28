package k8s

import (
	"fmt"

	"github.com/cloudfoundry-incubator/eirini"
	"github.com/cloudfoundry-incubator/eirini/opi"
	ext "k8s.io/api/extensions/v1beta1"
	"k8s.io/apimachinery/pkg/api/errors"
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
	CreateIngress(namespace string) (*ext.Ingress, error)
	UpdateIngress(namespace string, lrp opi.LRP, vcap VcapApp) error
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

func (i *KubeIngressManager) UpdateIngress(namespace string, lrp opi.LRP, vcap VcapApp) error {
	ingress, err := i.getIngress(namespace)
	if err != nil {
		return err
	}

	i.updateSpec(ingress, lrp, vcap)

	if _, err = i.client.ExtensionsV1beta1().Ingresses(namespace).Update(ingress); err != nil {
		return err
	}

	return nil
}

func (i *KubeIngressManager) updateSpec(ingress *ext.Ingress, lrp opi.LRP, vcap VcapApp) {
	newHost := fmt.Sprintf("%s.%s", vcap.AppName, i.endpoint)
	ingress.Spec.TLS[0].Hosts = append(ingress.Spec.TLS[0].Hosts, newHost)

	rule := createIngressRule(lrp, vcap, i.endpoint)
	ingress.Spec.Rules = append(ingress.Spec.Rules, rule)
}

func (i *KubeIngressManager) getIngress(namespace string) (*ext.Ingress, error) {
	ingress, err := i.client.ExtensionsV1beta1().Ingresses(namespace).Get(ingressName, av1.GetOptions{})

	if statusErr, ok := err.(*errors.StatusError); ok && statusErr.ErrStatus.Code == 404 {
		return i.CreateIngress(namespace)
	}
	return ingress, err
}

func createIngressRule(lrp opi.LRP, vcap VcapApp, kubeEndpoint string) ext.IngressRule {
	rule := ext.IngressRule{
		Host: fmt.Sprintf("%s.%s", vcap.AppName, kubeEndpoint),
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

func (i *KubeIngressManager) CreateIngress(namespace string) (*ext.Ingress, error) {
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

	return i.client.ExtensionsV1beta1().Ingresses(namespace).Create(ingress)
}

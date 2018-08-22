package k8s

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"strings"

	"code.cloudfoundry.org/eirini"
	"code.cloudfoundry.org/eirini/models/cf"
	"code.cloudfoundry.org/eirini/opi"
	"github.com/asaskevich/govalidator"
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
	Update(lrp *opi.LRP) error
	Delete(lrpName string) error
}

type KubeIngressManager struct {
	namespace string
	client    kubernetes.Interface
}

func NewIngressManager(client kubernetes.Interface, namespace string) IngressManager {
	return &KubeIngressManager{
		namespace: namespace,
		client:    client,
	}
}

func (i *KubeIngressManager) Delete(lrpName string) error {
	ing, err := i.client.ExtensionsV1beta1().Ingresses(i.namespace).Get(ingressName, av1.GetOptions{})
	if err != nil {
		return err
	}
	serviceName := eirini.GetInternalServiceName(lrpName)
	for i := 0; i < len(ing.Spec.Rules); i++ {
		if ing.Spec.Rules[i].HTTP.Paths[0].Backend.ServiceName == serviceName {
			ing.Spec.Rules = append(ing.Spec.Rules[:i], ing.Spec.Rules[i+1:]...)
			i--
		}
	}

	if len(ing.Spec.Rules) == 0 {
		err = i.client.ExtensionsV1beta1().Ingresses(i.namespace).Delete(ingressName, &av1.DeleteOptions{})
		return err
	}

	return i.updateIngressObject(ing)
}

func (i *KubeIngressManager) Update(lrp *opi.LRP) error {
	uriList := []string{}
	err := json.Unmarshal([]byte(lrp.Metadata[cf.VcapAppUris]), &uriList)
	if err != nil {
		panic(err)
	}

	ingresses, err := i.client.ExtensionsV1beta1().Ingresses(i.namespace).List(av1.ListOptions{})
	if err != nil {
		return err
	}

	if ingress, exists := i.getIngress(ingresses); exists {
		if len(uriList) == 0 {
			return i.Delete(lrp.Name)
		}

		if err := i.updateSpec(ingress, lrp.Name, uriList); err != nil {
			return err
		}
		return i.updateIngressObject(ingress)
	}

	return i.createIngress(lrp.Name, uriList)
}

func (i *KubeIngressManager) updateIngressObject(ingress *ext.Ingress) error {
	_, err := i.client.ExtensionsV1beta1().Ingresses(i.namespace).Update(ingress)
	return err
}

func (i *KubeIngressManager) createIngress(lrpName string, uriList []string) error {
	if len(uriList) == 0 {
		return nil
	}

	ingress := &ext.Ingress{
		TypeMeta: av1.TypeMeta{
			Kind:       ingressKind,
			APIVersion: ingressAPIVersion,
		},
		ObjectMeta: av1.ObjectMeta{
			Name:      ingressName,
			Namespace: i.namespace,
		},
		Spec: ext.IngressSpec{},
	}

	if err := i.updateSpec(ingress, lrpName, uriList); err != nil {
		return err
	}
	_, err := i.client.ExtensionsV1beta1().Ingresses(i.namespace).Create(ingress)
	return err
}

func (i *KubeIngressManager) updateSpec(ingress *ext.Ingress, lrpName string, uriList []string) error {
	rules, err := createIngressRules(lrpName, uriList)
	if err != nil {
		return err
	}

	ingress.Spec.Rules = removeDifference(
		ingress.Spec.Rules,
		rules,
		eirini.GetInternalServiceName(lrpName),
	)

	return nil
}

func (i *KubeIngressManager) getIngress(ingresses *ext.IngressList) (*ext.Ingress, bool) {
	for _, ing := range ingresses.Items {
		if ing.ObjectMeta.Name == ingressName {
			return &ing, true
		}
	}
	return &ext.Ingress{}, false
}

func createIngressRules(lrpName string, uriList []string) ([]ext.IngressRule, error) {
	rules := []ext.IngressRule{}

	for _, uri := range uriList {
		url, err := validateURL(uri)
		if err != nil {
			return nil, err
		}

		rule := ext.IngressRule{
			Host: strings.ToLower(url.Host),
		}

		rule.HTTP = &ext.HTTPIngressRuleValue{
			Paths: []ext.HTTPIngressPath{
				{
					Path: path(url.Path),
					Backend: ext.IngressBackend{
						ServiceName: eirini.GetInternalServiceName(lrpName),
						ServicePort: intstr.FromInt(8080),
					},
				},
			},
		}
		rules = append(rules, rule)
	}

	return rules, nil
}

func validateURL(uri string) (*url.URL, error) {
	url, err := url.Parse(fmt.Sprintf("http://%s", uri))
	if err != nil {
		return nil, err
	}

	if valid := govalidator.IsURL(url.String()); !valid {
		return nil, errors.New("invalid url")
	}

	return url, nil
}

func path(path string) string {
	if path == "" {
		return "/"
	}
	return path
}

func removeDifference(existing, updated []ext.IngressRule, serviceName string) []ext.IngressRule {
	hashExisting := toRuleMap(existing)
	hashUpdated := toRuleMap(updated)

	appendNewRules(hashExisting, hashUpdated)
	removeObsoleteRules(hashExisting, hashUpdated, serviceName)

	return toRuleSlice(hashExisting)
}

func appendNewRules(existing, updated map[ext.IngressRule]bool) {
	for u := range updated {
		if _, ok := existing[u]; !ok {
			existing[u] = true
		}
	}
}

func removeObsoleteRules(existing, updated map[ext.IngressRule]bool, serviceName string) {
	for e := range existing {
		if _, ok := updated[e]; !ok {
			if e.HTTP.Paths[0].Backend.ServiceName == serviceName {
				delete(existing, e)
			}
		}
	}
}

func toRuleMap(sl []ext.IngressRule) (hash map[ext.IngressRule]bool) {
	hash = map[ext.IngressRule]bool{}
	for _, v := range sl {
		hash[v] = true
	}
	return
}

func toRuleSlice(hash map[ext.IngressRule]bool) (sl []ext.IngressRule) {
	for v := range hash {
		sl = append(sl, v)
	}
	return
}

package k8s

import (
	"encoding/json"
	"regexp"

	"code.cloudfoundry.org/eirini"
	"code.cloudfoundry.org/eirini/models/cf"
	"code.cloudfoundry.org/eirini/opi"
	"code.cloudfoundry.org/eirini/route"
	"k8s.io/api/core/v1"
	meta_v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	types "k8s.io/client-go/kubernetes/typed/core/v1"
)

//go:generate counterfeiter . ServiceManager
type ServiceManager interface {
	route.Lister
	Create(lrp *opi.LRP) error
	CreateHeadless(lrp *opi.LRP) error
	Update(lrp *opi.LRP) error
	Delete(appName string) error
	DeleteHeadless(appName string) error
}

type serviceManager struct {
	client     kubernetes.Interface
	namespace  string
	routesChan chan<- []*eirini.Routes
}

func NewServiceManager(client kubernetes.Interface, namespace string, routesChan chan<- []*eirini.Routes) ServiceManager {
	return &serviceManager{
		client:     client,
		namespace:  namespace,
		routesChan: routesChan,
	}
}

func (m *serviceManager) services() types.ServiceInterface {
	return m.client.CoreV1().Services(m.namespace)
}

func (m *serviceManager) Create(lrp *opi.LRP) error {
	s, err := m.services().Create(toService(lrp))
	if err != nil {
		return err
	}

	registeredRoutes, err := decodeRoutes(s.Annotations[eirini.RegisteredRoutes])
	if err != nil {
		return err
	}

	routes := eirini.Routes{
		Routes: registeredRoutes,
		Name:   s.Name,
	}

	m.routesChan <- []*eirini.Routes{&routes}
	return nil
}

func (m *serviceManager) CreateHeadless(lrp *opi.LRP) error {
	_, err := m.services().Create(toHeadlessService(lrp))
	return err
}

func (m *serviceManager) Update(lrp *opi.LRP) error {
	serviceName := eirini.GetInternalServiceName(lrp.Name)
	service, err := m.services().Get(serviceName, meta_v1.GetOptions{})
	if err != nil {
		return err
	}

	registeredRoutes, err := decodeRoutes(service.Annotations[eirini.RegisteredRoutes])
	if err != nil {
		return err
	}
	updatedRoutes, err := decodeRoutes(lrp.Metadata[cf.VcapAppUris])
	if err != nil {
		return err
	}

	service.Annotations[eirini.RegisteredRoutes] = lrp.Metadata[cf.VcapAppUris]
	_, err = m.services().Update(service)
	if err != nil {
		return err
	}

	unregistered := getUnregisteredRoutes(registeredRoutes, updatedRoutes)
	routes := eirini.Routes{
		Routes:             updatedRoutes,
		UnregisteredRoutes: unregistered,
		Name:               serviceName,
	}

	m.routesChan <- []*eirini.Routes{&routes}
	return nil
}

func (m *serviceManager) Delete(appName string) error {
	serviceName := eirini.GetInternalServiceName(appName)
	service, err := m.services().Get(serviceName, meta_v1.GetOptions{})
	if err != nil {
		return err
	}

	existingRoutes, err := decodeRoutes(service.Annotations[eirini.RegisteredRoutes])
	if err != nil {
		return err
	}

	routes := eirini.Routes{
		UnregisteredRoutes: existingRoutes,
		Name:               serviceName,
	}

	m.routesChan <- []*eirini.Routes{&routes}

	return m.services().Delete(serviceName, &meta_v1.DeleteOptions{})
}

func (m *serviceManager) DeleteHeadless(appName string) error {
	serviceName := eirini.GetInternalHeadlessServiceName(appName)
	return m.services().Delete(serviceName, &meta_v1.DeleteOptions{})
}

func (m *serviceManager) ListRoutes() ([]*eirini.Routes, error) {
	services, err := m.services().List(meta_v1.ListOptions{})
	if err != nil {
		return []*eirini.Routes{}, err
	}

	routes := []*eirini.Routes{}
	for _, s := range services.Items {
		if !isCFService(s.Name) || isHeadless(s.Name) {
			continue
		}

		route := eirini.NewRoutes(m.removeUnregisteredRoutes)
		registered, err := decodeRoutes(s.Annotations[eirini.RegisteredRoutes])
		if err != nil {
			return []*eirini.Routes{}, err
		}

		route.Routes = registered
		route.Name = s.Name

		routes = append(routes, route)
	}

	return routes, nil
}

func (m *serviceManager) removeUnregisteredRoutes(serviceName string) error {
	service, err := m.services().Get(serviceName, meta_v1.GetOptions{})
	if err != nil {
		return err
	}

	if service.Annotations["delete"] == "true" {
		return m.services().Delete(serviceName, &meta_v1.DeleteOptions{})
	}

	service.Annotations[eirini.UnregisteredRoutes] = `[]`
	_, err = m.services().Update(service)

	return err
}

func toService(lrp *opi.LRP) *v1.Service {
	service := &v1.Service{
		Spec: v1.ServiceSpec{
			Ports: []v1.ServicePort{
				{
					Name: "service",
					Port: 8080,
				},
			},
			Selector: map[string]string{
				"name": lrp.Name,
			},
		},
	}

	service.Name = eirini.GetInternalServiceName(lrp.Name)
	service.Labels = map[string]string{
		"name": lrp.Name,
	}

	service.Annotations = map[string]string{
		eirini.RegisteredRoutes:   lrp.Metadata[cf.VcapAppUris],
		eirini.UnregisteredRoutes: `[]`,
	}

	return service
}

func toHeadlessService(lrp *opi.LRP) *v1.Service {
	service := &v1.Service{
		Spec: v1.ServiceSpec{
			ClusterIP: "None",
			Ports: []v1.ServicePort{
				{
					Name: "service",
					Port: 8080,
				},
			},
			Selector: map[string]string{
				"name": lrp.Name,
			},
		},
	}

	service.Name = eirini.GetInternalHeadlessServiceName(lrp.Name)
	service.Labels = map[string]string{
		"name": lrp.Name,
	}

	return service
}

func mergeRoutes(service *v1.Service) (string, error) {
	existingRoutes, err := decodeRoutes(service.Annotations[eirini.RegisteredRoutes])
	if err != nil {
		return "", err
	}

	unregisteredRoutes, err := decodeRoutes(service.Annotations[eirini.UnregisteredRoutes])
	if err != nil {
		return "", err
	}

	unregisteredRoutes = append(unregisteredRoutes, existingRoutes...)

	routes, err := json.Marshal(unregisteredRoutes)
	if err != nil {
		panic(err)
	}
	return string(routes), nil
}

func getUnregisteredRoutes(existing, updated []string) []string {
	updatedMap := sliceToMap(updated)
	unregistered := []string{}
	for _, e := range existing {
		if _, ok := updatedMap[e]; !ok {
			unregistered = append(unregistered, e)
		}
	}

	return unregistered
}

func sliceToMap(slice []string) map[string]bool {
	result := make(map[string]bool, len(slice))
	for _, e := range slice {
		result[e] = true
	}
	return result
}

func decodeRoutes(s string) ([]string, error) {
	uris := []string{}
	err := json.Unmarshal([]byte(s), &uris)

	return uris, err
}

func isHeadless(s string) bool {
	return matchRegex(s, "(headless)$")
}

func isCFService(s string) bool {
	return matchRegex(s, "^cf-.*$")
}

func matchRegex(subject string, regex string) bool {
	r, err := regexp.Compile(regex)
	if err != nil {
		panic(err)
	}
	return r.MatchString(subject)

}

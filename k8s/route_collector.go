package k8s

import (
	"context"
	"encoding/json"
	"fmt"

	"code.cloudfoundry.org/eirini/k8s/stset"
	"code.cloudfoundry.org/eirini/models/cf"
	"code.cloudfoundry.org/eirini/route"
	"code.cloudfoundry.org/lager"
	"github.com/pkg/errors"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
)

//counterfeiter:generate . StatefulSetGetter

type StatefulSetGetter interface {
	GetBySourceType(ctx context.Context, sourceType string) ([]appsv1.StatefulSet, error)
}

type RouteCollector struct {
	podsGetter        PodsGetter
	statefulSetGetter StatefulSetGetter
	logger            lager.Logger
}

func NewRouteCollector(podsGetter PodsGetter, statefulSetGetter StatefulSetGetter, logger lager.Logger) RouteCollector {
	return RouteCollector{
		podsGetter:        podsGetter,
		statefulSetGetter: statefulSetGetter,
		logger:            logger,
	}
}

func (c RouteCollector) Collect(ctx context.Context) ([]route.Message, error) {
	pods, err := c.podsGetter.GetAll(ctx)
	if err != nil {
		return nil, errors.Wrap(err, "failed to list pods")
	}

	statefulsets, err := c.getStatefulSets(ctx)
	if err != nil {
		return nil, err
	}

	routeMessages := []route.Message{}

	for _, p := range pods {
		routes, err := c.getRoutes(p, statefulsets)
		if err != nil {
			c.logger.Debug("collect.failed-to-get-routes", lager.Data{"error": err.Error()})

			continue
		}

		for _, r := range routes {
			routeMessage := route.Message{
				InstanceID: p.Name,
				Name:       p.Labels[stset.LabelGUID],
				Address:    p.Status.PodIP,
				Port:       uint32(r.Port),
				TLSPort:    0,
				Routes: route.Routes{
					RegisteredRoutes: []string{r.Hostname},
				},
			}
			routeMessages = append(routeMessages, routeMessage)
		}
	}

	return routeMessages, nil
}

func (c RouteCollector) getRoutes(pod corev1.Pod, statefulsets map[string]appsv1.StatefulSet) ([]cf.Route, error) {
	if !podReady(pod) {
		return nil, fmt.Errorf("pod %s is not ready", pod.Name)
	}

	ssName, err := getStatefulSetName(pod)
	if err != nil {
		return nil, fmt.Errorf("failed to get statefulset name for pod %s", pod.Name)
	}

	s, ok := statefulsets[ssName]

	if !ok {
		return nil, fmt.Errorf("statefulset for pod %s not found", pod.Name)
	}

	routeJSON, ok := s.Annotations[stset.AnnotationRegisteredRoutes]

	if !ok {
		return nil, fmt.Errorf("pod %s has no registered routes annotation", pod.Name)
	}

	var routes []cf.Route

	if json.Unmarshal([]byte(routeJSON), &routes) != nil {
		return nil, fmt.Errorf("failed to unmarshal routes for pod %s", pod.Name)
	}

	return routes, nil
}

func (c RouteCollector) getStatefulSets(ctx context.Context) (map[string]appsv1.StatefulSet, error) {
	statefulsetList, err := c.statefulSetGetter.GetBySourceType(ctx, stset.AppSourceType)
	if err != nil {
		return nil, errors.Wrap(err, "failed to list statefulsets")
	}

	statefulsetsMap := make(map[string]appsv1.StatefulSet)

	for _, s := range statefulsetList {
		statefulsetsMap[s.Name] = s
	}

	return statefulsetsMap, nil
}

func podReady(pod corev1.Pod) bool {
	for _, c := range pod.Status.Conditions {
		if c.Type == corev1.PodReady {
			return c.Status == corev1.ConditionTrue
		}
	}

	return false
}

func getStatefulSetName(pod corev1.Pod) (string, error) {
	if len(pod.OwnerReferences) == 0 {
		return "", fmt.Errorf("pod %s has no lowners", pod.Name)
	}

	for _, owner := range pod.OwnerReferences {
		if owner.Kind == "StatefulSet" {
			return owner.Name, nil
		}
	}

	return "", fmt.Errorf("pod %s doesn't have an owner statefulset", pod.Name)
}

package k8s

import (
	"fmt"

	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	meta "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func MapToEnvVar(env map[string]string) []v1.EnvVar {
	envVars := []v1.EnvVar{}
	for k, v := range env {
		envVar := v1.EnvVar{Name: k, Value: v}
		envVars = append(envVars, envVar)
	}
	return envVars
}

func int32ptr(i int) *int32 {
	u := int32(i)
	return &u
}

func int64ptr(i int) *int64 {
	u := int64(i)
	return &u
}

func GetEvents(client EventLister, pod v1.Pod) (*v1.EventList, error) {
	return client.List(meta.ListOptions{FieldSelector: fmt.Sprintf("involvedObject.namespace=%s,involvedObject.uid=%s,involvedObject.name=%s", pod.Namespace, string(pod.UID), pod.Name)})
}

func IsStopped(eventList *v1.EventList) bool {
	events := eventList.Items

	if len(events) == 0 {
		return false
	}

	event := events[len(events)-1]
	return event.Reason == eventKilling
}

func toCPUMillicores(cpuPercentage uint8) resource.Quantity {
	return *resource.NewScaledQuantity(int64(cpuPercentage)*10, resource.Milli) //nolint:gomnd
}

func toCPUPercentage(cpuMillicores int64) float64 {
	return float64(cpuMillicores) / 10
}

func toSeconds(millis uint) int32 {
	return int32(millis / 1000) //nolint:gomnd
}

package k8s

import (
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
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

func IsStopped(events []v1.Event) bool {
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

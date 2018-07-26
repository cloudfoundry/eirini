package k8s

import (
	"errors"
	"fmt"

	"k8s.io/api/core/v1"
)

func MergeMaps(maps ...map[string]string) map[string]string {
	result := make(map[string]string)
	for _, m := range maps {
		for k, v := range m {
			result[k] = v
		}
	}
	return result
}

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

// Enforce our assumption that there's only ever exactly one container holding the app.
func assertSingleContainer(containers []v1.Container) {
	if len(containers) != 1 {
		message := fmt.Sprintf("Unexpectedly, container count is not 1 but %d.", len(containers))
		panic(errors.New(message))
	}
}

func toMap(envVars []v1.EnvVar) map[string]string {
	result := make(map[string]string)
	for _, env := range envVars {
		result[env.Name] = env.Value
	}
	return result
}

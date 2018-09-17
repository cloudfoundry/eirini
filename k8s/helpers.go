package k8s

import (
	"k8s.io/api/core/v1"
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

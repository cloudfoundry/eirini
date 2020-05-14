package utils

import (
	"errors"

	v1 "k8s.io/api/core/v1"
)

func GetEnvVarValue(key string, vars []v1.EnvVar) (string, error) {
	for _, envVar := range vars {
		if envVar.Name == key {
			return envVar.Value, nil
		}
	}
	return "", errors.New("failed to find env var")
}

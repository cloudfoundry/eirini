package test

import (
	"io/ioutil"
	"os"
)

var CREDHUB_ENV_VARS []string = []string{"CREDHUB_SERVER", "CREDHUB_CLIENT", "CREDHUB_SECRET", "CREDHUB_CA_CERT"}

func UnsetAndCacheCredHubEnvVars() map[string]string {
	credhubEnv := make(map[string]string)
	for _, envVar := range CREDHUB_ENV_VARS {
		credhubEnv[envVar] = os.Getenv(envVar)
		os.Unsetenv(envVar)
	}
	return credhubEnv
}

func RestoreEnv(credhubEnv map[string]string) {
	for _, envVar := range CREDHUB_ENV_VARS {
		os.Setenv(envVar, credhubEnv[envVar])
	}
}
func CreateTempDir(prefix string) string {
	name, err := ioutil.TempDir("", prefix)
	if err != nil {
		panic(err)
	}
	return name
}

func CreateCredentialFile(dir, filename string, contents string) string {
	path := dir + "/" + filename
	err := ioutil.WriteFile(path, []byte(contents), 0644)
	if err != nil {
		panic(err)
	}
	return path
}

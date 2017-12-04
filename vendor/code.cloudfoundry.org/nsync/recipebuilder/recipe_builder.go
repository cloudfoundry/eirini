package recipebuilder

import (
	"fmt"

	"code.cloudfoundry.org/bbs/models"
	"code.cloudfoundry.org/diego-ssh/keys"
	"code.cloudfoundry.org/runtimeschema/cc_messages"
	"code.cloudfoundry.org/urljoiner"
)

const (
	MinCpuProxy = 128
	MaxCpuProxy = 8192

	DefaultFileDescriptorLimit = uint64(1024)

	LRPLogSource    = "CELL"
	AppLogSource    = "APP"
	HealthLogSource = "HEALTH"

	Router      = "router"
	DefaultPort = uint32(8080)

	DefaultSSHPort = uint32(2222)

	DefaultLANG = "en_US.UTF-8"

	TrustedSystemCertificatesPath = "/etc/cf-system-certificates"
)

var (
	ErrNoDockerImage        = Error{Type: "ErrNoDockerImage", Message: "no docker image provided"}
	ErrNoLifecycleDefined   = Error{Type: "ErrNoLifecycleDefined", Message: "no lifecycle binary bundle defined for stack"}
	ErrDropletSourceMissing = Error{Type: "ErrAppSourceMissing", Message: "desired app missing droplet_uri"}
	ErrDockerImageMissing   = Error{Type: "ErrDockerImageMissing", Message: "desired app missing docker_image"}
	ErrMultipleAppSources   = Error{Type: "ErrMultipleAppSources", Message: "desired app contains both droplet_uri and docker_image; exactly one is required."}
)

type Config struct {
	Lifecycles           map[string]string
	FileServerURL        string
	KeyFactory           keys.SSHKeyFactory
	PrivilegedContainers bool
}

//go:generate counterfeiter -o ../bulk/fakes/fake_recipe_builder.go . RecipeBuilder
type RecipeBuilder interface {
	Build(*cc_messages.DesireAppRequestFromCC) (*models.DesiredLRP, error)
	BuildTask(*cc_messages.TaskRequestFromCC) (*models.TaskDefinition, error)
	ExtractExposedPorts(*cc_messages.DesireAppRequestFromCC) ([]uint32, error)
}

type Error struct {
	Type    string `json:"name"`
	Message string `json:"message"`
}

func (err Error) Error() string {
	return err.Message
}

func lifecycleDownloadURL(lifecyclePath string, fileServerURL string) string {
	return urljoiner.Join(fileServerURL, "/v1/static", lifecyclePath)
}

func cpuWeight(memoryMB int) uint32 {
	cpuProxy := memoryMB

	if cpuProxy > MaxCpuProxy {
		return 100
	}

	if cpuProxy < MinCpuProxy {
		cpuProxy = MinCpuProxy
	}

	return uint32((100 * cpuProxy) / MaxCpuProxy)
}

func createLrpEnv(env []*models.EnvironmentVariable, exposedPorts []uint32, includeDeprecated bool) []*models.EnvironmentVariable {
	if len(exposedPorts) > 0 {
		portValue := fmt.Sprintf("%d", exposedPorts[0])
		env = append(env, &models.EnvironmentVariable{Name: "PORT", Value: portValue})
		if includeDeprecated {
			env = append(env, &models.EnvironmentVariable{Name: "VCAP_APP_PORT", Value: portValue})
		}
	}

	if includeDeprecated {
		env = append(env, &models.EnvironmentVariable{Name: "VCAP_APP_HOST", Value: "0.0.0.0"})
	}

	return env
}

func getParallelAction(ports []uint32, user string, uri string) *models.ParallelAction {
	fileDescriptorLimit := DefaultFileDescriptorLimit
	parallelAction := &models.ParallelAction{}
	for _, port := range ports {
		args := []string{fmt.Sprintf("-port=%d", port)}
		if uri != "" {
			args = append(args, fmt.Sprintf("-uri=%s", uri))
		}

		parallelAction.Actions = append(parallelAction.Actions,
			&models.Action{
				RunAction: &models.RunAction{
					User:      user,
					Path:      "/tmp/lifecycle/healthcheck",
					Args:      args,
					LogSource: HealthLogSource,
					ResourceLimits: &models.ResourceLimits{
						Nofile: &fileDescriptorLimit,
					},
					SuppressLogOutput: true,
				},
			})
	}
	return parallelAction
}

func getDesiredAppPorts(ports []uint32) []uint32 {
	desiredAppPorts := ports

	if desiredAppPorts == nil {
		desiredAppPorts = []uint32{DefaultPort}
	}
	return desiredAppPorts
}

func getAppLogSource(logSource string) string {
	if logSource == "" {
		return AppLogSource
	} else {
		return logSource
	}
}

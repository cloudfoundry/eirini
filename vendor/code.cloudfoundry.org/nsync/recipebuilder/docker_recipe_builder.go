package recipebuilder

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"strings"
	"time"

	"code.cloudfoundry.org/bbs/models"
	ssh_routes "code.cloudfoundry.org/diego-ssh/routes"
	"code.cloudfoundry.org/lager"
	"code.cloudfoundry.org/nsync/helpers"
	"code.cloudfoundry.org/runtimeschema/cc_messages"
)

const (
	DockerScheme      = "docker"
	DockerIndexServer = "docker.io"
)

type DockerRecipeBuilder struct {
	logger lager.Logger
	config Config
}

func NewDockerRecipeBuilder(logger lager.Logger, config Config) *DockerRecipeBuilder {
	return &DockerRecipeBuilder{
		logger: logger,
		config: config,
	}
}

func (b *DockerRecipeBuilder) BuildTask(task *cc_messages.TaskRequestFromCC) (*models.TaskDefinition, error) {
	logger := b.logger.Session("task-builder")

	var lifecycle = "docker"
	lifecyclePath, ok := b.config.Lifecycles[lifecycle]
	if !ok {
		logger.Error("unknown-lifecycle", ErrNoLifecycleDefined, lager.Data{
			"lifecycle": lifecycle,
		})

		return nil, ErrNoLifecycleDefined
	}

	lifecycleURL := lifecycleDownloadURL(lifecyclePath, b.config.FileServerURL)
	cachedDependencies := []*models.CachedDependency{
		{
			From:     lifecycleURL,
			To:       "/tmp/lifecycle",
			CacheKey: fmt.Sprintf("%s-lifecycle", strings.Replace(lifecycle, "/", "-", 1)),
		},
	}

	action := models.WrapAction(&models.RunAction{
		User:           "root",
		Path:           "/tmp/lifecycle/launcher",
		Args:           []string{"app", task.Command, "{}"},
		Env:            task.EnvironmentVariables,
		LogSource:      task.LogSource,
		ResourceLimits: &models.ResourceLimits{},
	})

	if task.DockerPath == "" {
		logger.Error("invalid-docker-path", ErrDockerImageMissing, lager.Data{"task": task})
		return nil, ErrDockerImageMissing
	}

	if task.DropletUri != "" {
		logger.Error("invalid-droplet-uri", ErrMultipleAppSources, lager.Data{"task": task})
		return nil, ErrMultipleAppSources
	}

	rootFSPath, err := convertDockerURI(task.DockerPath)
	if err != nil {
		return nil, err
	}

	placementTags := []string{}
	if task.IsolationSegment != "" {
		placementTags = []string{task.IsolationSegment}
	}

	taskDefinition := &models.TaskDefinition{
		LogGuid:               task.LogGuid,
		MemoryMb:              int32(task.MemoryMb),
		DiskMb:                int32(task.DiskMb),
		Privileged:            false,
		EnvironmentVariables:  task.EnvironmentVariables,
		EgressRules:           task.EgressRules,
		CompletionCallbackUrl: task.CompletionCallbackUrl,
		CachedDependencies:    cachedDependencies,
		LegacyDownloadUser:    "vcap",
		Action:                action,
		RootFs:                rootFSPath,
		TrustedSystemCertificatesPath: TrustedSystemCertificatesPath,
		LogSource:                     task.LogSource,
		VolumeMounts:                  convertVolumeMounts(task.VolumeMounts),
		PlacementTags:                 placementTags,
		ImageUsername:                 task.DockerUser,
		ImagePassword:                 task.DockerPassword,
	}

	return taskDefinition, nil
}

func (b *DockerRecipeBuilder) Build(desiredApp *cc_messages.DesireAppRequestFromCC) (*models.DesiredLRP, error) {
	lrpGuid := desiredApp.ProcessGuid

	buildLogger := b.logger.Session("message-builder")

	if desiredApp.DockerImageUrl == "" {
		buildLogger.Error("desired-app-invalid", ErrDockerImageMissing, lager.Data{"desired-app": desiredApp})
		return nil, ErrDockerImageMissing
	}

	if desiredApp.DropletUri != "" && desiredApp.DockerImageUrl != "" {
		buildLogger.Error("desired-app-invalid", ErrMultipleAppSources, lager.Data{"desired-app": desiredApp})
		return nil, ErrMultipleAppSources
	}

	var lifecycle = "docker"
	lifecyclePath, ok := b.config.Lifecycles[lifecycle]
	if !ok {
		buildLogger.Error("unknown-lifecycle", ErrNoLifecycleDefined, lager.Data{
			"lifecycle": lifecycle,
		})

		return nil, ErrNoLifecycleDefined
	}

	lifecycleURL := lifecycleDownloadURL(lifecyclePath, b.config.FileServerURL)

	rootFSPath := ""
	var err error
	rootFSPath, err = convertDockerURI(desiredApp.DockerImageUrl)
	if err != nil {
		return nil, err
	}

	var containerEnvVars []*models.EnvironmentVariable

	numFiles := DefaultFileDescriptorLimit
	if desiredApp.FileDescriptors != 0 {
		numFiles = desiredApp.FileDescriptors
	}

	var actions []models.ActionInterface
	var monitor models.ActionInterface

	executionMetadata, err := NewDockerExecutionMetadata(desiredApp.ExecutionMetadata)
	if err != nil {
		b.logger.Error("parsing-execution-metadata-failed", err, lager.Data{
			"desired-app-metadata": executionMetadata,
		})
		return nil, err
	}

	user, err := extractUser(executionMetadata)
	if err != nil {
		return nil, err
	}

	cachedDependencies := []*models.CachedDependency{}
	cachedDependencies = append(cachedDependencies, &models.CachedDependency{
		From:     lifecycleURL,
		To:       "/tmp/lifecycle",
		CacheKey: fmt.Sprintf("%s-lifecycle", strings.Replace(lifecycle, "/", "-", 1)),
	})

	desiredAppPorts, err := b.ExtractExposedPorts(desiredApp)
	if err != nil {
		return nil, err
	}

	switch desiredApp.HealthCheckType {
	case cc_messages.PortHealthCheckType, cc_messages.UnspecifiedHealthCheckType:
		monitor = models.Timeout(getParallelAction(desiredAppPorts, user, ""), 10*time.Minute)
	case cc_messages.HTTPHealthCheckType:
		monitor = models.Timeout(getParallelAction(desiredAppPorts, user, desiredApp.HealthCheckHTTPEndpoint), 10*time.Minute)
	}

	actions = append(actions, &models.RunAction{
		User: user,
		Path: "/tmp/lifecycle/launcher",
		Args: append(
			[]string{"app"},
			desiredApp.StartCommand,
			desiredApp.ExecutionMetadata,
		),
		Env:       createLrpEnv(desiredApp.Environment, desiredAppPorts, false),
		LogSource: getAppLogSource(desiredApp.LogSource),
		ResourceLimits: &models.ResourceLimits{
			Nofile: &numFiles,
		},
	})

	desiredAppRoutingInfo, err := helpers.CCRouteInfoToRoutes(desiredApp.RoutingInfo, desiredAppPorts)
	if err != nil {
		buildLogger.Error("marshaling-cc-route-info-failed", err)
		return nil, err
	}

	if desiredApp.AllowSSH {
		hostKeyPair, err := b.config.KeyFactory.NewKeyPair(1024)
		if err != nil {
			buildLogger.Error("new-host-key-pair-failed", err)
			return nil, err
		}

		userKeyPair, err := b.config.KeyFactory.NewKeyPair(1024)
		if err != nil {
			buildLogger.Error("new-user-key-pair-failed", err)
			return nil, err
		}

		actions = append(actions, &models.RunAction{
			User: user,
			Path: "/tmp/lifecycle/diego-sshd",
			Args: []string{
				"-address=" + fmt.Sprintf("0.0.0.0:%d", DefaultSSHPort),
				"-hostKey=" + hostKeyPair.PEMEncodedPrivateKey(),
				"-authorizedKey=" + userKeyPair.AuthorizedKey(),
				"-inheritDaemonEnv",
				"-logLevel=fatal",
			},
			Env: createLrpEnv(desiredApp.Environment, desiredAppPorts, false),
			ResourceLimits: &models.ResourceLimits{
				Nofile: &numFiles,
			},
		})

		sshRoutePayload, err := json.Marshal(ssh_routes.SSHRoute{
			ContainerPort:   2222,
			PrivateKey:      userKeyPair.PEMEncodedPrivateKey(),
			HostFingerprint: hostKeyPair.Fingerprint(),
		})

		if err != nil {
			buildLogger.Error("marshaling-ssh-route-failed", err)
			return nil, err
		}

		sshRouteMessage := json.RawMessage(sshRoutePayload)
		desiredAppRoutingInfo[ssh_routes.DIEGO_SSH] = &sshRouteMessage
		desiredAppPorts = append(desiredAppPorts, DefaultSSHPort)
	}

	actionAction := models.Codependent(actions...)

	placementTags := []string{}
	if desiredApp.IsolationSegment != "" {
		placementTags = []string{desiredApp.IsolationSegment}
	}

	return &models.DesiredLRP{
		Privileged: false,

		Domain: cc_messages.AppLRPDomain,

		ProcessGuid: lrpGuid,
		Instances:   int32(desiredApp.NumInstances),
		Routes:      &desiredAppRoutingInfo,
		Annotation:  desiredApp.ETag,

		CpuWeight: cpuWeight(desiredApp.MemoryMB),

		MemoryMb: int32(desiredApp.MemoryMB),
		DiskMb:   int32(desiredApp.DiskMB),

		Ports: desiredAppPorts,

		RootFs: rootFSPath,

		LogGuid:   desiredApp.LogGuid,
		LogSource: LRPLogSource,

		MetricsGuid: desiredApp.LogGuid,

		EnvironmentVariables: containerEnvVars,
		CachedDependencies:   cachedDependencies,
		Action:               models.WrapAction(actionAction),
		Monitor:              models.WrapAction(monitor),

		StartTimeoutMs: int64(desiredApp.HealthCheckTimeoutInSeconds * 1000),

		EgressRules:        desiredApp.EgressRules,
		Network:            desiredApp.Network,
		LegacyDownloadUser: user,

		TrustedSystemCertificatesPath: TrustedSystemCertificatesPath,
		VolumeMounts:                  convertVolumeMounts(desiredApp.VolumeMounts),
		PlacementTags:                 placementTags,

		ImageUsername: desiredApp.DockerUser,
		ImagePassword: desiredApp.DockerPassword,
	}, nil
}

func (b DockerRecipeBuilder) ExtractExposedPorts(desiredApp *cc_messages.DesireAppRequestFromCC) ([]uint32, error) {
	if len(desiredApp.Ports) > 0 {
		return desiredApp.Ports, nil
	}

	executionMetadata := desiredApp.ExecutionMetadata
	metadata, err := NewDockerExecutionMetadata(executionMetadata)
	if err != nil {
		return nil, err
	}
	return extractExposedPorts(metadata, b.logger)
}

func extractExposedPorts(executionMetadata DockerExecutionMetadata, logger lager.Logger) ([]uint32, error) {
	var exposedPort uint32 = DefaultPort
	exposedPorts := executionMetadata.ExposedPorts
	ports := make([]uint32, 0)
	if len(exposedPorts) == 0 {
		ports = append(ports, exposedPort)
	}
	for _, port := range exposedPorts {
		if port.Protocol == "tcp" {
			exposedPort = port.Port
			ports = append(ports, exposedPort)
		}
	}

	if len(ports) == 0 {
		err := fmt.Errorf("No tcp ports found in image metadata")
		logger.Error("parsing-exposed-ports-failed", err, lager.Data{
			"desired-app-metadata": executionMetadata,
		})
		return nil, err
	}

	return ports, nil
}

func extractUser(executionMetadata DockerExecutionMetadata) (string, error) {
	if len(executionMetadata.User) > 0 {
		return executionMetadata.User, nil
	} else {
		return "root", nil
	}
}

func convertDockerURI(dockerURI string) (string, error) {
	if strings.Contains(dockerURI, "://") {
		return "", errors.New("docker URI [" + dockerURI + "] should not contain scheme")
	}

	indexName, remoteName, tag := parseDockerRepoUrl(dockerURI)

	return (&url.URL{
		Scheme:   DockerScheme,
		Path:     indexName + "/" + remoteName,
		Fragment: tag,
	}).String(), nil
}

// via https://github.com/docker/docker/blob/a271eaeba224652e3a12af0287afbae6f82a9333/registry/config.go#L295
func parseDockerRepoUrl(dockerURI string) (indexName, remoteName, tag string) {
	nameParts := strings.SplitN(dockerURI, "/", 2)

	if officialRegistry(nameParts) {
		// URI without host
		indexName = ""
		remoteName = dockerURI

		// URI has format docker.io/<path>
		if nameParts[0] == DockerIndexServer {
			indexName = DockerIndexServer
			remoteName = nameParts[1]
		}

		// Remote name contain no '/' - prefix it with "library/"
		// via https://github.com/docker/docker/blob/a271eaeba224652e3a12af0287afbae6f82a9333/registry/config.go#L343
		if strings.IndexRune(remoteName, '/') == -1 {
			remoteName = "library/" + remoteName
		}
	} else {
		indexName = nameParts[0]
		remoteName = nameParts[1]
	}

	remoteName, tag = parseDockerRepositoryTag(remoteName)

	return indexName, remoteName, tag
}

func officialRegistry(nameParts []string) bool {
	return len(nameParts) == 1 ||
		nameParts[0] == DockerIndexServer ||
		(!strings.Contains(nameParts[0], ".") &&
			!strings.Contains(nameParts[0], ":") &&
			nameParts[0] != "localhost")
}

// via https://github.com/docker/docker/blob/4398108/pkg/parsers/parsers.go#L72
func parseDockerRepositoryTag(remoteName string) (string, string) {
	n := strings.LastIndex(remoteName, ":")
	if n < 0 {
		return remoteName, ""
	}
	if tag := remoteName[n+1:]; !strings.Contains(tag, "/") {
		return remoteName[:n], tag
	}
	return remoteName, ""
}

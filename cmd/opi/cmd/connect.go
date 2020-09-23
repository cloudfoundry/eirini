package cmd

import (
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
	"path/filepath"

	"code.cloudfoundry.org/eirini"
	"code.cloudfoundry.org/eirini/bifrost"
	"code.cloudfoundry.org/eirini/bifrost/namespacers"
	cmdcommons "code.cloudfoundry.org/eirini/cmd"
	"code.cloudfoundry.org/eirini/handler"
	"code.cloudfoundry.org/eirini/k8s"
	"code.cloudfoundry.org/eirini/k8s/client"
	"code.cloudfoundry.org/eirini/stager"
	"code.cloudfoundry.org/eirini/stager/docker"
	"code.cloudfoundry.org/eirini/util"
	"code.cloudfoundry.org/lager"
	"code.cloudfoundry.org/tlsconfig"
	"github.com/pkg/errors"
	"github.com/spf13/cobra"
	yaml "gopkg.in/yaml.v2"
	"k8s.io/client-go/kubernetes"

	// For gcp and oidc authentication
	_ "k8s.io/client-go/plugin/pkg/client/auth"
)

func connect(cmd *cobra.Command, args []string) {
	path, err := cmd.Flags().GetString("config")
	cmdcommons.ExitIfError(err)

	if path == "" {
		cmdcommons.Exitf("--config is missing")
	}

	cfg := setConfigFromFile(path)
	clientset := cmdcommons.CreateKubeClient(cfg.Properties.ConfigPath)

	buildpackStagingBifrost := initBuildpackStagingBifrost(cfg, clientset)
	dockerStagingBifrost := initDockerStagingBifrost(cfg)
	taskBifrost := initTaskBifrost(cfg, clientset)
	bifrost := initLRPBifrost(clientset, cfg)

	handlerLogger := lager.NewLogger("handler")
	handlerLogger.RegisterSink(lager.NewPrettySink(os.Stdout, lager.DEBUG))
	handler := handler.New(bifrost, buildpackStagingBifrost, dockerStagingBifrost, taskBifrost, handlerLogger)
	handlerLogger.Info("opi-connected")

	if cfg.Properties.ServePlaintext {
		servePlaintext(cfg, handler, handlerLogger)
	}

	serveTLS(cfg, handler, handlerLogger)
}

func serveTLS(cfg *eirini.Config, handler http.Handler, logger lager.Logger) {
	var server *http.Server

	crtPath := cmdcommons.GetOrDefault(cfg.Properties.ServerCertPath, eirini.EiriniCrtPath)
	keyPath := cmdcommons.GetOrDefault(cfg.Properties.ServerKeyPath, eirini.EiriniKeyPath)
	caPath := cmdcommons.GetOrDefault(cfg.Properties.ClientCAPath, eirini.EiriniCAPath)

	tlsConfig, err := tlsconfig.Build(
		tlsconfig.WithInternalServiceDefaults(),
	).Server(
		tlsconfig.WithClientAuthenticationFromFile(caPath),
	)
	cmdcommons.ExitIfError(err)

	server = &http.Server{
		Addr:      fmt.Sprintf("0.0.0.0:%d", cfg.Properties.TLSPort),
		Handler:   handler,
		TLSConfig: tlsConfig,
	}
	logger.Fatal("opi-crashed",
		server.ListenAndServeTLS(crtPath, keyPath))
}

func servePlaintext(cfg *eirini.Config, handler http.Handler, logger lager.Logger) {
	server := &http.Server{
		Addr:    fmt.Sprintf("0.0.0.0:%d", cfg.Properties.PlaintextPort),
		Handler: handler,
	}
	logger.Fatal("opi-crashed", server.ListenAndServe())
}

func initRetryableJSONClient(cfg *eirini.Config) *util.RetryableJSONClient {
	httpClient := http.DefaultClient

	if !cfg.Properties.CCTLSDisabled {
		crtPath := cmdcommons.GetOrDefault(cfg.Properties.CCCertPath, eirini.CCCrtPath)
		keyPath := cmdcommons.GetOrDefault(cfg.Properties.CCKeyPath, eirini.CCKeyPath)
		caPath := cmdcommons.GetOrDefault(cfg.Properties.CCCAPath, eirini.CCCAPath)

		var err error
		httpClient, err = util.CreateTLSHTTPClient(
			[]util.CertPaths{
				{
					Crt: crtPath,
					Key: keyPath,
					Ca:  caPath,
				},
			},
		)

		if err != nil {
			panic(errors.Wrap(err, "failed to create stager http client"))
		}
	}

	return util.NewRetryableJSONClient(httpClient)
}

func initStagingCompleter(cfg *eirini.Config, logger lager.Logger) *stager.CallbackStagingCompleter {
	retryableJSONClient := initRetryableJSONClient(cfg)

	return stager.NewCallbackStagingCompleter(logger, retryableJSONClient)
}

func initTaskDesirer(cfg *eirini.Config, clientset kubernetes.Interface) *k8s.TaskDesirer {
	tlsConfigs := []k8s.StagingConfigTLS{
		{
			SecretName: eirini.CCUploaderSecretName,
			KeyPaths: []k8s.KeyPath{
				{Key: eirini.TLSSecretKey, Path: eirini.CCAPIKeyName},
				{Key: eirini.TLSSecretCert, Path: eirini.CCAPICertName},
			},
		},
		{
			SecretName: eirini.EiriniClientSecretName,
			KeyPaths: []k8s.KeyPath{
				{Key: eirini.TLSSecretKey, Path: eirini.EiriniClientKey},
				{Key: eirini.TLSSecretCert, Path: eirini.EiriniClientCert},
			},
		},
		{
			SecretName: eirini.EiriniClientSecretName,
			KeyPaths: []k8s.KeyPath{
				{Key: eirini.TLSSecretCA, Path: eirini.CACertName},
			},
		},
	}

	logger := lager.NewLogger("task-desirer")
	logger.RegisterSink(lager.NewPrettySink(os.Stdout, lager.DEBUG))

	return k8s.NewTaskDesirer(
		logger,
		client.NewJob(clientset, cfg.Properties.Namespace, cfg.Properties.EnableMultiNamespaceSupport),
		client.NewSecret(clientset),
		cfg.Properties.Namespace,
		tlsConfigs,
		cfg.Properties.ApplicationServiceAccount,
		cfg.Properties.StagingServiceAccount,
		cfg.Properties.RegistrySecretName,
		cfg.Properties.UnsafeAllowAutomountServiceAccountToken,
	)
}

func initTaskDeleter(clientset kubernetes.Interface, jobClient k8s.JobDeletingClient) *k8s.TaskDeleter {
	logger := lager.NewLogger("task-desirer")
	logger.RegisterSink(lager.NewPrettySink(os.Stdout, lager.DEBUG))

	return k8s.NewTaskDeleter(
		logger,
		jobClient,
		client.NewSecret(clientset),
	)
}

func initBuildpackStagingBifrost(cfg *eirini.Config, clientset kubernetes.Interface) *bifrost.BuildpackStaging {
	logger := lager.NewLogger("buildpack-staging-bifrost")
	logger.RegisterSink(lager.NewPrettySink(os.Stdout, lager.DEBUG))

	converter := initConverter(cfg)
	taskDesirer := initTaskDesirer(cfg, clientset)
	stagingJobClient := client.NewStagingJob(clientset, cfg.Properties.Namespace, cfg.Properties.EnableMultiNamespaceSupport)
	taskDeleter := initTaskDeleter(clientset, stagingJobClient)
	stagingCompleter := initStagingCompleter(cfg, logger)

	return &bifrost.BuildpackStaging{
		Converter:        converter,
		StagingDesirer:   taskDesirer,
		StagingDeleter:   taskDeleter,
		StagingCompleter: stagingCompleter,
		Logger:           logger,
	}
}

func initDockerStagingBifrost(cfg *eirini.Config) *bifrost.DockerStaging {
	logger := lager.NewLogger("docker-staging-bifrost")
	logger.RegisterSink(lager.NewPrettySink(os.Stdout, lager.DEBUG))
	stagingCompleter := initStagingCompleter(cfg, logger)

	return &bifrost.DockerStaging{
		Logger:               logger,
		ImageMetadataFetcher: docker.Fetch,
		ImageRefParser:       docker.Parse,
		StagingCompleter:     stagingCompleter,
	}
}

func initTaskBifrost(cfg *eirini.Config, clientset kubernetes.Interface) *bifrost.Task {
	converter := initConverter(cfg)
	taskDesirer := initTaskDesirer(cfg, clientset)
	jobClient := client.NewJob(clientset, cfg.Properties.Namespace, cfg.Properties.EnableMultiNamespaceSupport)
	taskDeleter := initTaskDeleter(clientset, jobClient)
	retryableJSONClient := initRetryableJSONClient(cfg)
	namespacer := initNamespacer(cfg)

	return &bifrost.Task{
		Converter:   converter,
		TaskDesirer: taskDesirer,
		TaskDeleter: taskDeleter,
		JSONClient:  retryableJSONClient,
		Namespacer:  namespacer,
	}
}

func setConfigFromFile(path string) *eirini.Config {
	fileBytes, err := ioutil.ReadFile(filepath.Clean(path))
	cmdcommons.ExitIfError(err)

	var conf eirini.Config
	err = yaml.Unmarshal(fileBytes, &conf)
	cmdcommons.ExitIfError(err)

	return &conf
}

func initLRPBifrost(clientset kubernetes.Interface, cfg *eirini.Config) *bifrost.LRP {
	desireLogger := lager.NewLogger("desirer")
	desireLogger.RegisterSink(lager.NewPrettySink(os.Stdout, lager.DEBUG))

	desirer := &k8s.StatefulSetDesirer{
		Pods:                              client.NewPod(clientset, cfg.Properties.Namespace, cfg.Properties.EnableMultiNamespaceSupport),
		Secrets:                           client.NewSecret(clientset),
		StatefulSets:                      client.NewStatefulSet(clientset, cfg.Properties.Namespace, cfg.Properties.EnableMultiNamespaceSupport),
		PodDisruptionBudgets:              client.NewPodDisruptionBudget(clientset),
		EventsClient:                      client.NewEvent(clientset, cfg.Properties.Namespace, cfg.Properties.EnableMultiNamespaceSupport),
		StatefulSetToLRPMapper:            k8s.StatefulSetToLRP,
		RegistrySecretName:                cfg.Properties.RegistrySecretName,
		LivenessProbeCreator:              k8s.CreateLivenessProbe,
		ReadinessProbeCreator:             k8s.CreateReadinessProbe,
		Logger:                            desireLogger,
		ApplicationServiceAccount:         cfg.Properties.ApplicationServiceAccount,
		AllowAutomountServiceAccountToken: cfg.Properties.UnsafeAllowAutomountServiceAccountToken,
	}
	converter := initConverter(cfg)
	namespacer := initNamespacer(cfg)

	return &bifrost.LRP{
		Converter:  converter,
		Desirer:    desirer,
		Namespacer: namespacer,
	}
}

func initConverter(cfg *eirini.Config) *bifrost.OPIConverter {
	convertLogger := lager.NewLogger("convert")
	convertLogger.RegisterSink(lager.NewPrettySink(os.Stdout, lager.DEBUG))

	stagerCfg := eirini.StagerConfig{
		EiriniAddress:   cfg.Properties.EiriniAddress,
		DownloaderImage: cfg.Properties.DownloaderImage,
		UploaderImage:   cfg.Properties.UploaderImage,
		ExecutorImage:   cfg.Properties.ExecutorImage,
	}

	return bifrost.NewOPIConverter(
		convertLogger,
		cfg.Properties.RegistryAddress,
		docker.Fetch,
		docker.Parse,
		cfg.Properties.AllowRunImageAsRoot,
		stagerCfg,
	)
}

func initNamespacer(cfg *eirini.Config) bifrost.LRPNamespacer {
	if cfg.Properties.EnableMultiNamespaceSupport {
		return namespacers.NewMultiNamespace(cfg.Properties.Namespace)
	}

	return namespacers.NewSingleNamespace(cfg.Properties.Namespace)
}

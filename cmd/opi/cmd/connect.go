package cmd

import (
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
	"path/filepath"

	"code.cloudfoundry.org/eirini"
	"code.cloudfoundry.org/eirini/bifrost"
	cmdcommons "code.cloudfoundry.org/eirini/cmd"
	"code.cloudfoundry.org/eirini/handler"
	"code.cloudfoundry.org/eirini/k8s"
	"code.cloudfoundry.org/eirini/stager"
	"code.cloudfoundry.org/eirini/stager/docker"
	"code.cloudfoundry.org/eirini/util"
	"code.cloudfoundry.org/lager"
	"github.com/pkg/errors"
	"github.com/spf13/cobra"
	yaml "gopkg.in/yaml.v2"

	"k8s.io/client-go/kubernetes"

	// For gcp and oidc authentication
	_ "k8s.io/client-go/plugin/pkg/client/auth"

	"code.cloudfoundry.org/tlsconfig"
)

var connectCmd = &cobra.Command{
	Use:   "connect",
	Short: "connects CloudFoundry with Kubernetes",
	Run:   connect,
}

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

	tlsConfig, err := tlsconfig.Build(
		tlsconfig.WithInternalServiceDefaults(),
	).Server(
		tlsconfig.WithClientAuthenticationFromFile(cfg.Properties.ClientCAPath),
	)
	cmdcommons.ExitIfError(err)

	server = &http.Server{
		Addr:      fmt.Sprintf("0.0.0.0:%d", cfg.Properties.TLSPort),
		Handler:   handler,
		TLSConfig: tlsConfig,
	}
	logger.Fatal("opi-crashed",
		server.ListenAndServeTLS(cfg.Properties.ServerCertPath, cfg.Properties.ServerKeyPath))
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
		var err error
		httpClient, err = util.CreateTLSHTTPClient(
			[]util.CertPaths{
				{
					Crt: cfg.Properties.CCCertPath,
					Key: cfg.Properties.CCKeyPath,
					Ca:  cfg.Properties.CCCAPath,
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
			SecretName: cfg.Properties.CCUploaderSecretName,
			KeyPaths: []k8s.KeyPath{
				{Key: cfg.Properties.CCUploaderKeyPath, Path: eirini.CCAPIKeyName},
				{Key: cfg.Properties.CCUploaderCertPath, Path: eirini.CCAPICertName},
			},
		},
		{
			SecretName: cfg.Properties.ClientCertsSecretName,
			KeyPaths: []k8s.KeyPath{
				{Key: cfg.Properties.ClientCertPath, Path: eirini.EiriniClientCert},
				{Key: cfg.Properties.ClientKeyPath, Path: eirini.EiriniClientKey},
			},
		},
		{
			SecretName: cfg.Properties.CACertSecretName,
			KeyPaths: []k8s.KeyPath{
				{Key: cfg.Properties.CACertPath, Path: eirini.CACertName},
			},
		},
	}

	logger := lager.NewLogger("task-desirer")
	logger.RegisterSink(lager.NewPrettySink(os.Stdout, lager.DEBUG))

	return k8s.NewTaskDesirerWithEiriniInstance(
		logger,
		k8s.NewJobClient(clientset),
		k8s.NewSecretsClient(clientset),
		cfg.Properties.Namespace,
		tlsConfigs,
		cfg.Properties.ApplicationServiceAccount,
		cfg.Properties.StagingServiceAccount,
		cfg.Properties.RegistrySecretName,
		cfg.Properties.RootfsVersion,
		cfg.Properties.EiriniInstance,
	)
}

func initTaskDeleter(clientset kubernetes.Interface, eiriniInstance string) *k8s.TaskDeleter {
	logger := lager.NewLogger("task-desirer")
	logger.RegisterSink(lager.NewPrettySink(os.Stdout, lager.DEBUG))

	return k8s.NewTaskDeleter(
		logger,
		k8s.NewJobClient(clientset),
		k8s.NewSecretsClient(clientset),
		eiriniInstance,
	)
}

func initBuildpackStagingBifrost(cfg *eirini.Config, clientset kubernetes.Interface) *bifrost.BuildpackStaging {
	logger := lager.NewLogger("buildpack-staging-bifrost")
	logger.RegisterSink(lager.NewPrettySink(os.Stdout, lager.DEBUG))
	converter := initConverter(cfg)
	taskDesirer := initTaskDesirer(cfg, clientset)
	taskDeleter := initTaskDeleter(clientset, cfg.Properties.EiriniInstance)
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
	taskDeleter := initTaskDeleter(clientset, cfg.Properties.EiriniInstance)
	retryableJSONClient := initRetryableJSONClient(cfg)
	return &bifrost.Task{
		DefaultNamespace: cfg.Properties.Namespace,
		Converter:        converter,
		TaskDesirer:      taskDesirer,
		TaskDeleter:      taskDeleter,
		JSONClient:       retryableJSONClient,
	}
}

func setConfigFromFile(path string) *eirini.Config {
	fileBytes, err := ioutil.ReadFile(filepath.Clean(path))
	cmdcommons.ExitIfError(err)

	var conf eirini.Config
	conf.Properties.DiskLimitMB = 2048
	err = yaml.Unmarshal(fileBytes, &conf)
	cmdcommons.ExitIfError(err)

	return &conf
}

func initLRPBifrost(clientset kubernetes.Interface, cfg *eirini.Config) *bifrost.LRP {
	desireLogger := lager.NewLogger("desirer")
	desireLogger.RegisterSink(lager.NewPrettySink(os.Stdout, lager.DEBUG))
	desirer := &k8s.StatefulSetDesirer{
		Pods:                              k8s.NewPodsClient(clientset),
		Secrets:                           k8s.NewSecretsClient(clientset),
		StatefulSets:                      k8s.NewStatefulSetClient(clientset),
		PodDisruptionBudets:               k8s.NewPodDisruptionBudgetClient(clientset),
		Events:                            k8s.NewEventsClient(clientset),
		StatefulSetToLRPMapper:            k8s.StatefulSetToLRP,
		RegistrySecretName:                cfg.Properties.RegistrySecretName,
		RootfsVersion:                     cfg.Properties.RootfsVersion,
		LivenessProbeCreator:              k8s.CreateLivenessProbe,
		ReadinessProbeCreator:             k8s.CreateReadinessProbe,
		Hasher:                            util.TruncatedSHA256Hasher{},
		Logger:                            desireLogger,
		ApplicationServiceAccount:         cfg.Properties.ApplicationServiceAccount,
		AllowAutomountServiceAccountToken: cfg.Properties.UnsafeAllowAutomountServiceAccountToken,
	}
	converter := initConverter(cfg)

	return &bifrost.LRP{
		DefaultNamespace: cfg.Properties.Namespace,
		Converter:        converter,
		Desirer:          desirer,
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
		cfg.Properties.DiskLimitMB,
		docker.Fetch,
		docker.Parse,
		cfg.Properties.AllowRunImageAsRoot,
		stagerCfg,
	)
}

func initConnect() {
	connectCmd.Flags().StringP("config", "c", "", "Path to the Eirini config file")
}

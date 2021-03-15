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
	"code.cloudfoundry.org/eirini/k8s/client"
	"code.cloudfoundry.org/eirini/k8s/jobs"
	"code.cloudfoundry.org/eirini/k8s/pdb"
	"code.cloudfoundry.org/eirini/k8s/stset"
	"code.cloudfoundry.org/eirini/stager"
	"code.cloudfoundry.org/eirini/stager/docker"
	"code.cloudfoundry.org/eirini/util"
	"code.cloudfoundry.org/lager"
	"code.cloudfoundry.org/tlsconfig"
	"github.com/spf13/cobra"
	yaml "gopkg.in/yaml.v2"
	"k8s.io/client-go/kubernetes"

	// For gcp and oidc authentication
	_ "k8s.io/client-go/plugin/pkg/client/auth"
)

func connect(cmd *cobra.Command, args []string) {
	path, err := cmd.Flags().GetString("config")
	cmdcommons.ExitfIfError(err, "Failed to get config flag")

	if path == "" {
		cmdcommons.Exitf("--config is missing")
	}

	cfg := setConfigFromFile(path)
	clientset := cmdcommons.CreateKubeClient(cfg.ConfigPath)

	stSetClient := client.NewStatefulSet(clientset, cfg.WorkloadsNamespace)
	provider := cmdcommons.CreateMigrationStepsProvider(stSetClient, cfg.WorkloadsNamespace)

	dockerStagingBifrost := initDockerStagingBifrost(cfg)
	taskBifrost := initTaskBifrost(cfg, clientset)
	bifrost := initLRPBifrost(clientset, cfg, provider.GetLatestMigrationIndex())

	handlerLogger := lager.NewLogger("handler")
	handlerLogger.RegisterSink(lager.NewPrettySink(os.Stdout, lager.DEBUG))
	handler := handler.New(bifrost, dockerStagingBifrost, taskBifrost, handlerLogger)
	handlerLogger.Info("opi-connected")

	if cfg.ServePlaintext {
		servePlaintext(cfg, handler, handlerLogger)
	}

	serveTLS(cfg, handler, handlerLogger)
}

func serveTLS(cfg *eirini.APIConfig, handler http.Handler, logger lager.Logger) {
	var server *http.Server

	crtPath, keyPath, caPath := cmdcommons.GetCertPaths(eirini.EnvServerCertDir, eirini.EiriniCrtDir, "Eirini Server")

	tlsConfig, err := tlsconfig.Build(
		tlsconfig.WithInternalServiceDefaults(),
	).Server(
		tlsconfig.WithClientAuthenticationFromFile(caPath),
	)
	cmdcommons.ExitfIfError(err, "Failed to build TLS config")

	server = &http.Server{
		Addr:      fmt.Sprintf("0.0.0.0:%d", cfg.TLSPort),
		Handler:   handler,
		TLSConfig: tlsConfig,
	}
	logger.Fatal("opi-crashed",
		server.ListenAndServeTLS(crtPath, keyPath))
}

func servePlaintext(cfg *eirini.APIConfig, handler http.Handler, logger lager.Logger) {
	server := &http.Server{
		Addr:    fmt.Sprintf("0.0.0.0:%d", cfg.PlaintextPort),
		Handler: handler,
	}
	logger.Fatal("opi-crashed", server.ListenAndServe())
}

func initRetryableJSONClient(cfg *eirini.APIConfig) *util.RetryableJSONClient {
	httpClient := http.DefaultClient

	if !cfg.CCTLSDisabled {
		crtPath, keyPath, caPath := cmdcommons.GetCertPaths(eirini.EnvCCCertDir, eirini.CCCrtDir, "Cloud Controller")

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
			cmdcommons.ExitfIfError(err, "failed to create stager http client")
		}
	}

	return util.NewRetryableJSONClient(httpClient)
}

func initStagingCompleter(cfg *eirini.APIConfig, logger lager.Logger) *stager.CallbackStagingCompleter {
	retryableJSONClient := initRetryableJSONClient(cfg)

	return stager.NewCallbackStagingCompleter(logger, retryableJSONClient)
}

func initTaskClient(cfg *eirini.APIConfig, clientset kubernetes.Interface) *k8s.TaskClient {
	logger := lager.NewLogger("task-desirer")
	logger.RegisterSink(lager.NewPrettySink(os.Stdout, lager.DEBUG))

	taskToJobConverter := jobs.NewTaskToJobConverter(
		cfg.ApplicationServiceAccount,
		cfg.RegistrySecretName,
		cfg.UnsafeAllowAutomountServiceAccountToken,
	)

	return k8s.NewTaskClient(
		logger,
		client.NewJob(clientset, cfg.WorkloadsNamespace),
		client.NewSecret(clientset),
		taskToJobConverter,
	)
}

func initDockerStagingBifrost(cfg *eirini.APIConfig) *bifrost.DockerStaging {
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

func initTaskBifrost(cfg *eirini.APIConfig, clientset kubernetes.Interface) *bifrost.Task {
	converter := initConverter(cfg)
	taskClient := initTaskClient(cfg, clientset)
	retryableJSONClient := initRetryableJSONClient(cfg)
	namespacer := bifrost.NewNamespacer(cfg.DefaultWorkloadsNamespace)

	return &bifrost.Task{
		Converter:  converter,
		TaskClient: taskClient,
		JSONClient: retryableJSONClient,
		Namespacer: namespacer,
	}
}

func setConfigFromFile(path string) *eirini.APIConfig {
	fileBytes, err := ioutil.ReadFile(filepath.Clean(path))
	cmdcommons.ExitfIfError(err, "Failed to read config file")

	var conf eirini.APIConfig
	err = yaml.Unmarshal(fileBytes, &conf)
	cmdcommons.ExitfIfError(err, "Failed to unmarshal config file")

	return &conf
}

func initLRPBifrost(clientset kubernetes.Interface, cfg *eirini.APIConfig, latestMigration int) *bifrost.LRP {
	desireLogger := lager.NewLogger("desirer")
	desireLogger.RegisterSink(lager.NewPrettySink(os.Stdout, lager.DEBUG))

	lrpToStatefulSetConverter := stset.NewLRPToStatefulSetConverter(
		cfg.ApplicationServiceAccount,
		cfg.RegistrySecretName,
		cfg.UnsafeAllowAutomountServiceAccountToken,
		cfg.AllowRunImageAsRoot,
		latestMigration,
		k8s.CreateLivenessProbe,
		k8s.CreateReadinessProbe,
	)
	lrpClient := k8s.NewLRPClient(
		desireLogger,
		client.NewSecret(clientset),
		client.NewStatefulSet(clientset, cfg.WorkloadsNamespace),
		client.NewPod(clientset, cfg.WorkloadsNamespace),
		pdb.NewCreatorDeleter(client.NewPodDisruptionBudget(clientset)),
		client.NewEvent(clientset),
		lrpToStatefulSetConverter,
		stset.NewStatefulSetToLRPConverter(),
	)

	converter := initConverter(cfg)
	namespacer := bifrost.NewNamespacer(cfg.DefaultWorkloadsNamespace)

	return &bifrost.LRP{
		Converter:  converter,
		LRPClient:  lrpClient,
		Namespacer: namespacer,
	}
}

func initConverter(cfg *eirini.APIConfig) *bifrost.OPIConverter {
	convertLogger := lager.NewLogger("convert")
	convertLogger.RegisterSink(lager.NewPrettySink(os.Stdout, lager.DEBUG))

	return bifrost.NewOPIConverter(
		convertLogger,
	)
}

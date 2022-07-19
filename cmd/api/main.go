package main

import (
	"fmt"
	"net/http"
	"os"
	"time"

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
	"github.com/jessevdk/go-flags"
	"k8s.io/client-go/kubernetes"
)

const readHaderTimeout = 30 * time.Second

type options struct {
	ConfigFile string `short:"c" long:"config" description:"Config for running the eirini api"`
}

func main() {
	var opts options
	_, err := flags.ParseArgs(&opts, os.Args)
	cmdcommons.ExitfIfError(err, "Failed to parse args")

	var cfg eirini.APIConfig
	err = cmdcommons.ReadConfigFile(opts.ConfigFile, &cfg)
	cmdcommons.ExitfIfError(err, "Failed to read config file")

	clientset := cmdcommons.CreateKubeClient(cfg.ConfigPath)

	latestMigrationIndex := cmdcommons.GetLatestMigrationIndex()

	dockerStagingBifrost := initDockerStagingBifrost(cfg)
	taskBifrost := initTaskBifrost(cfg, clientset, latestMigrationIndex)
	bifrost := initLRPBifrost(clientset, cfg, latestMigrationIndex)

	handlerLogger := lager.NewLogger("handler")
	handlerLogger.RegisterSink(lager.NewPrettySink(os.Stdout, lager.DEBUG))
	handler := handler.New(bifrost, dockerStagingBifrost, taskBifrost, handlerLogger)
	handlerLogger.Info("api-connected")

	if cfg.ServePlaintext {
		servePlaintext(cfg, handler, handlerLogger)
	}

	serveTLS(cfg, handler, handlerLogger)
}

func serveTLS(cfg eirini.APIConfig, handler http.Handler, logger lager.Logger) {
	var server *http.Server

	crtPath, keyPath, caPath := cmdcommons.GetCertPaths(eirini.EnvServerCertDir, eirini.EiriniCrtDir, "Eirini Server")

	tlsConfig, err := tlsconfig.Build(
		tlsconfig.WithInternalServiceDefaults(),
	).Server(
		tlsconfig.WithClientAuthenticationFromFile(caPath),
	)
	cmdcommons.ExitfIfError(err, "Failed to build TLS config")

	server = &http.Server{
		Addr:              fmt.Sprintf("0.0.0.0:%d", cfg.TLSPort),
		Handler:           handler,
		TLSConfig:         tlsConfig,
		ReadHeaderTimeout: readHaderTimeout,
	}
	logger.Fatal("api-crashed",
		server.ListenAndServeTLS(crtPath, keyPath))
}

func servePlaintext(cfg eirini.APIConfig, handler http.Handler, logger lager.Logger) {
	server := &http.Server{
		Addr:              fmt.Sprintf("0.0.0.0:%d", cfg.PlaintextPort),
		Handler:           handler,
		ReadHeaderTimeout: readHaderTimeout,
	}
	logger.Fatal("api-crashed", server.ListenAndServe())
}

func initRetryableJSONClient(cfg eirini.APIConfig) *util.RetryableJSONClient {
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

func initStagingCompleter(cfg eirini.APIConfig, logger lager.Logger) *stager.CallbackStagingCompleter {
	retryableJSONClient := initRetryableJSONClient(cfg)

	return stager.NewCallbackStagingCompleter(logger, retryableJSONClient)
}

func initTaskClient(cfg eirini.APIConfig, clientset kubernetes.Interface, latestMigrationIndex int) *k8s.TaskClient {
	logger := lager.NewLogger("task-desirer")
	logger.RegisterSink(lager.NewPrettySink(os.Stdout, lager.DEBUG))

	taskToJobConverter := jobs.NewTaskToJobConverter(
		cfg.ApplicationServiceAccount,
		cfg.RegistrySecretName,
		cfg.UnsafeAllowAutomountServiceAccountToken,
		latestMigrationIndex,
	)

	return k8s.NewTaskClient(
		logger,
		client.NewJob(clientset, cfg.WorkloadsNamespace),
		client.NewSecret(clientset),
		taskToJobConverter,
	)
}

func initDockerStagingBifrost(cfg eirini.APIConfig) *bifrost.DockerStaging {
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

func initTaskBifrost(cfg eirini.APIConfig, clientset kubernetes.Interface, latestMigrationIndex int) *bifrost.Task {
	converter := initConverter(cfg)
	taskClient := initTaskClient(cfg, clientset, latestMigrationIndex)
	retryableJSONClient := initRetryableJSONClient(cfg)
	namespacer := bifrost.NewNamespacer(cfg.DefaultWorkloadsNamespace)

	return &bifrost.Task{
		Converter:  converter,
		TaskClient: taskClient,
		JSONClient: retryableJSONClient,
		Namespacer: namespacer,
	}
}

func initLRPBifrost(clientset kubernetes.Interface, cfg eirini.APIConfig, latestMigration int) *bifrost.LRP {
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
		pdb.NewUpdater(client.NewPodDisruptionBudget(clientset)),
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

func initConverter(cfg eirini.APIConfig) *bifrost.APIConverter {
	convertLogger := lager.NewLogger("convert")
	convertLogger.RegisterSink(lager.NewPrettySink(os.Stdout, lager.DEBUG))

	return bifrost.NewAPIConverter(
		convertLogger,
	)
}

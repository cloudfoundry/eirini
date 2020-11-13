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
	clientset := cmdcommons.CreateKubeClient(cfg.Properties.ConfigPath)

	dockerStagingBifrost := initDockerStagingBifrost(cfg)
	taskBifrost := initTaskBifrost(cfg, clientset)
	bifrost := initLRPBifrost(clientset, cfg)

	handlerLogger := lager.NewLogger("handler")
	handlerLogger.RegisterSink(lager.NewPrettySink(os.Stdout, lager.DEBUG))
	handler := handler.New(bifrost, dockerStagingBifrost, taskBifrost, handlerLogger)
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
	cmdcommons.ExitfIfError(err, "Failed to build TLS config")

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
		crtPath := cmdcommons.GetExistingFile(cfg.Properties.CCCertPath, eirini.CCCrtPath, "CC Cert")
		keyPath := cmdcommons.GetExistingFile(cfg.Properties.CCKeyPath, eirini.CCKeyPath, "CC Key")
		caPath := cmdcommons.GetExistingFile(cfg.Properties.CCCAPath, eirini.CCCAPath, "CC CA")

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

func initStagingCompleter(cfg *eirini.Config, logger lager.Logger) *stager.CallbackStagingCompleter {
	retryableJSONClient := initRetryableJSONClient(cfg)

	return stager.NewCallbackStagingCompleter(logger, retryableJSONClient)
}

func initTaskDesirer(cfg *eirini.Config, clientset kubernetes.Interface) *k8s.TaskDesirer {
	logger := lager.NewLogger("task-desirer")
	logger.RegisterSink(lager.NewPrettySink(os.Stdout, lager.DEBUG))

	return k8s.NewTaskDesirer(
		logger,
		client.NewJob(clientset, cfg.WorkloadsNamespace),
		client.NewSecret(clientset),
		cfg.Properties.ApplicationServiceAccount,
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
	jobClient := client.NewJob(clientset, cfg.WorkloadsNamespace)
	taskDeleter := initTaskDeleter(clientset, jobClient)
	retryableJSONClient := initRetryableJSONClient(cfg)
	namespacer := bifrost.NewNamespacer(cfg.Properties.DefaultWorkloadsNamespace)

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
	cmdcommons.ExitfIfError(err, "Failed to read config file")

	var conf eirini.Config
	err = yaml.Unmarshal(fileBytes, &conf)
	cmdcommons.ExitfIfError(err, "Failed to unmarshal config file")

	return &conf
}

func initLRPBifrost(clientset kubernetes.Interface, cfg *eirini.Config) *bifrost.LRP {
	desireLogger := lager.NewLogger("desirer")
	desireLogger.RegisterSink(lager.NewPrettySink(os.Stdout, lager.DEBUG))

	desirer := &k8s.StatefulSetDesirer{
		Pods:                              client.NewPod(clientset, cfg.WorkloadsNamespace),
		Secrets:                           client.NewSecret(clientset),
		StatefulSets:                      client.NewStatefulSet(clientset, cfg.WorkloadsNamespace),
		PodDisruptionBudgets:              client.NewPodDisruptionBudget(clientset),
		EventsClient:                      client.NewEvent(clientset),
		StatefulSetToLRPMapper:            k8s.StatefulSetToLRP,
		RegistrySecretName:                cfg.Properties.RegistrySecretName,
		LivenessProbeCreator:              k8s.CreateLivenessProbe,
		ReadinessProbeCreator:             k8s.CreateReadinessProbe,
		Logger:                            desireLogger,
		ApplicationServiceAccount:         cfg.Properties.ApplicationServiceAccount,
		AllowAutomountServiceAccountToken: cfg.Properties.UnsafeAllowAutomountServiceAccountToken,
	}
	converter := initConverter(cfg)
	namespacer := bifrost.NewNamespacer(cfg.Properties.DefaultWorkloadsNamespace)

	return &bifrost.LRP{
		Converter:  converter,
		Desirer:    desirer,
		Namespacer: namespacer,
	}
}

func initConverter(cfg *eirini.Config) *bifrost.OPIConverter {
	convertLogger := lager.NewLogger("convert")
	convertLogger.RegisterSink(lager.NewPrettySink(os.Stdout, lager.DEBUG))

	return bifrost.NewOPIConverter(
		convertLogger,
		docker.Fetch,
		docker.Parse,
		cfg.Properties.AllowRunImageAsRoot,
	)
}

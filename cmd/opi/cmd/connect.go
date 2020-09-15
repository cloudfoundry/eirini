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
	"github.com/pkg/errors"
	"github.com/spf13/cobra"
	yaml "gopkg.in/yaml.v2"
	"k8s.io/client-go/kubernetes"

	// For gcp and oidc authentication
	_ "k8s.io/client-go/plugin/pkg/client/auth"
)

const (
	TLSSecretKey  = "tls.key"
	TLSSecretCert = "tls.crt"
	TLSSecretCA   = "ca.crt"

	EiriniCAPath  = "/etc/eirini/certs/ca.crt"
	EiriniCrtPath = "/etc/eirini/certs/tls.crt"
	EiriniKeyPath = "/etc/eirini/certs/tls.key"
	CCCrtPath     = "/etc/cf-api/certs/tls.crt"
	CCKeyPath     = "/etc/cf-api/certs/tls.key"
	CCCAPath      = "/etc/cf-api/certs/ca.crt"

	CCUploaderSecretName   = "cc-uploader-certs"   //#nosec G101
	EiriniClientSecretName = "eirini-client-certs" //#nosec G101
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

	crtPath := getWithDefault(cfg.Properties.ServerCertPath, EiriniCrtPath)
	keyPath := getWithDefault(cfg.Properties.ServerKeyPath, EiriniKeyPath)
	caPath := getWithDefault(cfg.Properties.ClientCAPath, EiriniCAPath)

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
		crtPath := getWithDefault(cfg.Properties.CCCertPath, CCCrtPath)
		keyPath := getWithDefault(cfg.Properties.CCKeyPath, CCKeyPath)
		caPath := getWithDefault(cfg.Properties.CCCAPath, CCCAPath)

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
			SecretName: CCUploaderSecretName,
			KeyPaths: []k8s.KeyPath{
				{Key: TLSSecretKey, Path: eirini.CCAPIKeyName},
				{Key: TLSSecretCert, Path: eirini.CCAPICertName},
			},
		},
		{
			SecretName: EiriniClientSecretName,
			KeyPaths: []k8s.KeyPath{
				{Key: TLSSecretKey, Path: eirini.EiriniClientKey},
				{Key: TLSSecretCert, Path: eirini.EiriniClientCert},
			},
		},
		{
			SecretName: EiriniClientSecretName,
			KeyPaths: []k8s.KeyPath{
				{Key: TLSSecretCA, Path: eirini.CACertName},
			},
		},
	}

	logger := lager.NewLogger("task-desirer")
	logger.RegisterSink(lager.NewPrettySink(os.Stdout, lager.DEBUG))

	return k8s.NewTaskDesirer(
		logger,
		client.NewJob(clientset),
		client.NewSecret(clientset),
		cfg.Properties.Namespace,
		tlsConfigs,
		cfg.Properties.ApplicationServiceAccount,
		cfg.Properties.StagingServiceAccount,
		cfg.Properties.RegistrySecretName,
		cfg.Properties.UnsafeAllowAutomountServiceAccountToken,
	)
}

func initTaskDeleter(clientset kubernetes.Interface, jobClient k8s.JobClient) *k8s.TaskDeleter {
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
	stagingJobClient := client.NewStagingJob(clientset)
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
	jobClient := client.NewJob(clientset)
	taskDeleter := initTaskDeleter(clientset, jobClient)
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
	err = yaml.Unmarshal(fileBytes, &conf)
	cmdcommons.ExitIfError(err)

	return &conf
}

func initLRPBifrost(clientset kubernetes.Interface, cfg *eirini.Config) *bifrost.LRP {
	desireLogger := lager.NewLogger("desirer")
	desireLogger.RegisterSink(lager.NewPrettySink(os.Stdout, lager.DEBUG))

	desirer := &k8s.StatefulSetDesirer{
		Pods:                              client.NewPod(clientset),
		Secrets:                           client.NewSecret(clientset),
		StatefulSets:                      client.NewStatefulSet(clientset),
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
		docker.Fetch,
		docker.Parse,
		cfg.Properties.AllowRunImageAsRoot,
		stagerCfg,
	)
}

func getWithDefault(actualValue, defaultValue string) string {
	if actualValue != "" {
		return actualValue
	}

	return defaultValue
}

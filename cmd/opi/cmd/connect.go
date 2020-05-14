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
		cmdcommons.ExitIfError(errors.New("--config is missing"))
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

	var server *http.Server
	handlerLogger.Info("opi-connected")

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
	handlerLogger.Fatal("opi-crashed",
		server.ListenAndServeTLS(cfg.Properties.ServerCertPath, cfg.Properties.ServerKeyPath))
}

func initStagingCompleter(cfg *eirini.Config, logger lager.Logger) *stager.CallbackStagingCompleter {
	httpClient, err := util.CreateTLSHTTPClient(
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

	return stager.NewCallbackStagingCompleter(logger, httpClient)
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

	return &k8s.TaskDesirer{
		Namespace:          cfg.Properties.Namespace,
		TLSConfig:          tlsConfigs,
		ServiceAccountName: cfg.Properties.ApplicationServiceAccount,
		RegistrySecretName: cfg.Properties.RegistrySecretName,
		SecretsClient:      clientset.CoreV1().Secrets(cfg.Properties.Namespace),
		JobClient:          clientset.BatchV1().Jobs(cfg.Properties.Namespace),
		Logger:             logger,
	}
}

func initBuildpackStagingBifrost(cfg *eirini.Config, clientset kubernetes.Interface) *bifrost.BuildpackStaging {
	logger := lager.NewLogger("buildpack-staging-bifrost")
	logger.RegisterSink(lager.NewPrettySink(os.Stdout, lager.DEBUG))
	converter := initConverter(cfg)
	taskDesirer := initTaskDesirer(cfg, clientset)
	stagingCompleter := initStagingCompleter(cfg, logger)

	return &bifrost.BuildpackStaging{
		Converter:        converter,
		StagingDesirer:   taskDesirer,
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
	return &bifrost.Task{
		Converter:   converter,
		TaskDesirer: taskDesirer,
	}
}

func initLRPBifrost(clientset kubernetes.Interface, cfg *eirini.Config) *bifrost.LRP {
	kubeNamespace := cfg.Properties.Namespace
	desireLogger := lager.NewLogger("desirer")
	desireLogger.RegisterSink(lager.NewPrettySink(os.Stdout, lager.DEBUG))
	desirer := k8s.NewStatefulSetDesirer(
		clientset,
		kubeNamespace,
		cfg.Properties.RegistrySecretName,
		cfg.Properties.RootfsVersion,
		cfg.Properties.ApplicationServiceAccount,
		desireLogger,
	)
	converter := initConverter(cfg)

	return &bifrost.LRP{
		Converter: converter,
		Desirer:   desirer,
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

func setConfigFromFile(path string) *eirini.Config {
	fileBytes, err := ioutil.ReadFile(filepath.Clean(path))
	cmdcommons.ExitIfError(err)

	var conf eirini.Config
	conf.Properties.DiskLimitMB = 2048
	err = yaml.Unmarshal(fileBytes, &conf)
	cmdcommons.ExitIfError(err)

	return &conf
}

func initConnect() {
	connectCmd.Flags().StringP("config", "c", "", "Path to the Eirini config file")
}

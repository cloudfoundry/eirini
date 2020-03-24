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

	stagerLogger := lager.NewLogger("stager")
	stagerLogger.RegisterSink(lager.NewPrettySink(os.Stdout, lager.DEBUG))

	stagingCompleter := initStagingCompleter(cfg, stagerLogger)

	clientset := cmdcommons.CreateKubeClient(cfg.Properties.ConfigPath)
	buildpackStager := initBuildpackStager(clientset, cfg, stagingCompleter, stagerLogger)
	dockerStager := initDockerStager(stagingCompleter, stagerLogger)
	bifrost := initBifrost(clientset, cfg)

	handlerLogger := lager.NewLogger("handler")
	handlerLogger.RegisterSink(lager.NewPrettySink(os.Stdout, lager.DEBUG))
	handler := handler.New(bifrost, buildpackStager, dockerStager, nil, handlerLogger)

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

func initStagingCompleter(cfg *eirini.Config, logger lager.Logger) stager.StagingCompleter {
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

func initBuildpackStager(clientset kubernetes.Interface, cfg *eirini.Config, stagingCompleter stager.StagingCompleter, logger lager.Logger) eirini.Stager {
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

	taskDesirer := &k8s.TaskDesirer{
		Namespace:          cfg.Properties.Namespace,
		TLSConfig:          tlsConfigs,
		ServiceAccountName: cfg.Properties.ApplicationServiceAccount,
		JobClient:          clientset.BatchV1().Jobs(cfg.Properties.Namespace),
	}

	stagerCfg := eirini.StagerConfig{
		EiriniAddress:   cfg.Properties.EiriniAddress,
		DownloaderImage: cfg.Properties.DownloaderImage,
		UploaderImage:   cfg.Properties.UploaderImage,
		ExecutorImage:   cfg.Properties.ExecutorImage,
	}

	return stager.New(taskDesirer, stagingCompleter, stagerCfg, logger)
}

func initDockerStager(stagingCompleter stager.StagingCompleter, logger lager.Logger) eirini.Stager {
	return docker.Stager{
		Logger:               logger,
		ImageMetadataFetcher: docker.Fetch,
		StagingCompleter:     stagingCompleter,
	}
}

func initBifrost(clientset kubernetes.Interface, cfg *eirini.Config) eirini.Bifrost {
	kubeNamespace := cfg.Properties.Namespace
	desireLogger := lager.NewLogger("desirer")
	desireLogger.RegisterSink(lager.NewPrettySink(os.Stdout, lager.DEBUG))
	desirer := k8s.NewStatefulSetDesirer(clientset, kubeNamespace, cfg.Properties.RegistrySecretName, cfg.Properties.RootfsVersion, cfg.Properties.ApplicationServiceAccount, cfg.Properties.ApplicationPrivilegedServiceAccount, desireLogger)
	convertLogger := lager.NewLogger("convert")
	convertLogger.RegisterSink(lager.NewPrettySink(os.Stdout, lager.DEBUG))
	registryIP := cfg.Properties.RegistryAddress
	converter := bifrost.NewConverter(convertLogger, registryIP, cfg.Properties.DiskLimitMB)

	return &bifrost.Bifrost{
		Converter: converter,
		Desirer:   desirer,
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

func initConnect() {
	connectCmd.Flags().StringP("config", "c", "", "Path to the Eirini config file")
}

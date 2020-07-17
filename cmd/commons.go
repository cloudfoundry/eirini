package cmd

import (
	"fmt"
	"os"

	"code.cloudfoundry.org/eirini"
	"code.cloudfoundry.org/eirini/bifrost"
	"code.cloudfoundry.org/eirini/k8s"
	"code.cloudfoundry.org/eirini/stager/docker"
	"code.cloudfoundry.org/eirini/util"
	"code.cloudfoundry.org/lager"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/klog"
	metricsclientset "k8s.io/metrics/pkg/client/clientset/versioned"

	// Kubernetes has a tricky way to add authentication
	_ "k8s.io/client-go/plugin/pkg/client/auth"
)

func CreateMetricsClient(kubeConfigPath string) metricsclientset.Interface {
	klog.SetOutput(os.Stdout)
	klog.SetOutputBySeverity("Fatal", os.Stderr)
	config, err := clientcmd.BuildConfigFromFlags("", kubeConfigPath)
	ExitIfError(err)

	metricsClient, err := metricsclientset.NewForConfig(config)
	ExitIfError(err)

	return metricsClient
}

func CreateKubeClient(kubeConfigPath string) kubernetes.Interface {
	klog.SetOutput(os.Stdout)
	klog.SetOutputBySeverity("Fatal", os.Stderr)
	config, err := clientcmd.BuildConfigFromFlags("", kubeConfigPath)
	ExitIfError(err)

	clientset, err := kubernetes.NewForConfig(config)
	ExitIfError(err)

	return clientset
}

func ExitIfError(err error) {
	if err != nil {
		panic(err)
	}
}

func Exitf(messageFormat string, args ...interface{}) {
	panic(fmt.Sprintf(messageFormat, args...))
}

func InitLRPBifrost(clientset kubernetes.Interface, cfg *eirini.Config) *bifrost.LRP {
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
	converter := InitConverter(cfg)

	return &bifrost.LRP{
		DefaultNamespace: cfg.Properties.Namespace,
		Converter:        converter,
		Desirer:          desirer,
	}
}

func InitConverter(cfg *eirini.Config) *bifrost.OPIConverter {
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

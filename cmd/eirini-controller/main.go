package main

import (
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"

	"code.cloudfoundry.org/eirini"
	cmdcommons "code.cloudfoundry.org/eirini/cmd"
	"code.cloudfoundry.org/eirini/k8s"
	"code.cloudfoundry.org/eirini/k8s/client"
	"code.cloudfoundry.org/eirini/k8s/reconciler"
	eirinischeme "code.cloudfoundry.org/eirini/pkg/generated/clientset/versioned/scheme"
	"code.cloudfoundry.org/lager"
	"github.com/jessevdk/go-flags"
	"github.com/pkg/errors"
	"gopkg.in/yaml.v2"
	ctrl "sigs.k8s.io/controller-runtime"

	eiriniv1 "code.cloudfoundry.org/eirini/pkg/apis/eirini/v1"
	appsv1 "k8s.io/api/apps/v1"
	batchv1 "k8s.io/api/batch/v1"
	"k8s.io/client-go/kubernetes"
	kscheme "k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/tools/clientcmd"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	runtimeclient "sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/config"
	"sigs.k8s.io/controller-runtime/pkg/manager"
)

type options struct {
	ConfigFile string `short:"c" long:"config" description:"Config for running eirini-controller"`
}

func main() {
	err := kscheme.AddToScheme(eirinischeme.Scheme)
	cmdcommons.ExitIfError(fmt.Errorf("failed to add the k8s scheme to the LRP CRD scheme: %w", err))

	var opts options
	_, err = flags.ParseArgs(&opts, os.Args)
	cmdcommons.ExitIfError(err)

	eiriniCfg, err := readConfigFile(opts.ConfigFile)
	cmdcommons.ExitIfError(err)

	kubeConfig, err := clientcmd.BuildConfigFromFlags("", eiriniCfg.Properties.ConfigPath)
	cmdcommons.ExitIfError(err)

	controllerClient, err := runtimeclient.New(kubeConfig, runtimeclient.Options{Scheme: eirinischeme.Scheme})
	cmdcommons.ExitIfError(err)

	clientset, err := kubernetes.NewForConfig(kubeConfig)
	cmdcommons.ExitIfError(err)

	logger := lager.NewLogger("eirini-informer")
	logger.RegisterSink(lager.NewPrettySink(os.Stdout, lager.DEBUG))

	stDesirer := &k8s.StatefulSetDesirer{
		Pods:                              client.NewPod(clientset),
		Secrets:                           client.NewSecret(clientset),
		StatefulSets:                      client.NewStatefulSet(clientset),
		PodDisruptionBudets:               client.NewPodDisruptionBudget(clientset),
		Events:                            client.NewEvent(clientset),
		StatefulSetToLRPMapper:            k8s.StatefulSetToLRP,
		RegistrySecretName:                eiriniCfg.Properties.RegistrySecretName,
		RootfsVersion:                     eiriniCfg.Properties.RootfsVersion,
		LivenessProbeCreator:              k8s.CreateLivenessProbe,
		ReadinessProbeCreator:             k8s.CreateReadinessProbe,
		Logger:                            logger.Session("statefulset-desirer"),
		ApplicationServiceAccount:         eiriniCfg.Properties.ApplicationServiceAccount,
		AllowAutomountServiceAccountToken: eiriniCfg.Properties.UnsafeAllowAutomountServiceAccountToken,
	}

	taskDesirer := k8s.NewTaskDesirer(
		logger.Session("task-desirer"),
		client.NewJob(clientset),
		client.NewSecret(clientset),
		"",
		[]k8s.StagingConfigTLS{},
		eiriniCfg.Properties.ApplicationServiceAccount,
		"",
		eiriniCfg.Properties.RegistrySecretName,
		eiriniCfg.Properties.RootfsVersion,
	)

	mgr, err := manager.New(config.GetConfigOrDie(), manager.Options{
		Scheme: eirinischeme.Scheme,
	})
	cmdcommons.ExitIfError(err)
	lrpReconciler := reconciler.NewLRP(logger.Session("lrp-reconciler"), controllerClient, stDesirer, client.NewStatefulSet(clientset), mgr.GetScheme())
	taskReconciler := reconciler.NewTask(logger.Session("task-reconciler"), controllerClient, taskDesirer, mgr.GetScheme())

	err = builder.
		ControllerManagedBy(mgr).
		For(&eiriniv1.LRP{}).
		Owns(&appsv1.StatefulSet{}).
		Complete(lrpReconciler)
	cmdcommons.ExitIfError(err)

	err = builder.
		ControllerManagedBy(mgr).
		For(&eiriniv1.Task{}).
		Owns(&batchv1.Job{}).
		Complete(taskReconciler)
	cmdcommons.ExitIfError(err)

	err = mgr.Start(ctrl.SetupSignalHandler())
	cmdcommons.ExitIfError(err)
}

func readConfigFile(path string) (*eirini.Config, error) {
	fileBytes, err := ioutil.ReadFile(filepath.Clean(path))
	if err != nil {
		return nil, errors.Wrap(err, "failed to read file")
	}

	var conf eirini.Config
	err = yaml.Unmarshal(fileBytes, &conf)
	return &conf, errors.Wrap(err, "failed to unmarshal yaml")
}

package main

import (
	"io/ioutil"
	"os"
	"path/filepath"

	"code.cloudfoundry.org/eirini"
	cmdcommons "code.cloudfoundry.org/eirini/cmd"
	"code.cloudfoundry.org/eirini/k8s"
	"code.cloudfoundry.org/eirini/k8s/client"
	eirinievent "code.cloudfoundry.org/eirini/k8s/informers/event"
	"code.cloudfoundry.org/eirini/k8s/reconciler"
	eiriniv1 "code.cloudfoundry.org/eirini/pkg/apis/eirini/v1"
	eirinischeme "code.cloudfoundry.org/eirini/pkg/generated/clientset/versioned/scheme"
	"code.cloudfoundry.org/eirini/util"
	"code.cloudfoundry.org/lager"
	"github.com/jessevdk/go-flags"
	"github.com/pkg/errors"
	"gopkg.in/yaml.v2"
	appsv1 "k8s.io/api/apps/v1"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes"
	kscheme "k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/tools/clientcmd"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	runtimeclient "sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
)

type options struct {
	ConfigFile string `short:"c" long:"config" description:"Config for running eirini-controller"`
}

func main() {
	if err := kscheme.AddToScheme(eirinischeme.Scheme); err != nil {
		cmdcommons.Exitf("failed to add the k8s scheme to the LRP CRD scheme: %v", err)
	}

	var opts options
	_, err := flags.ParseArgs(&opts, os.Args)
	cmdcommons.ExitfIfError(err, "Failed to parse args")

	eiriniCfg, err := readConfigFile(opts.ConfigFile)
	cmdcommons.ExitfIfError(err, "Failed to read config file")

	kubeConfig, err := clientcmd.BuildConfigFromFlags("", eiriniCfg.Properties.ConfigPath)
	cmdcommons.ExitfIfError(err, "Failed to build kubeconfig")

	controllerClient, err := runtimeclient.New(kubeConfig, runtimeclient.Options{Scheme: eirinischeme.Scheme})
	cmdcommons.ExitfIfError(err, "Failed to create k8s runtime client")

	clientset, err := kubernetes.NewForConfig(kubeConfig)
	cmdcommons.ExitfIfError(err, "Failed to create k8s clientset")

	logger := lager.NewLogger("eirini-controller")
	logger.RegisterSink(lager.NewPrettySink(os.Stdout, lager.DEBUG))

	managerOptions := manager.Options{
		// do not serve prometheus metrics; disabled because port clashes during integration tests
		MetricsBindAddress: "0",
		Scheme:             eirinischeme.Scheme,
		Logger:             util.NewLagerLogr(logger),
	}

	mgr, err := manager.New(kubeConfig, managerOptions)
	cmdcommons.ExitfIfError(err, "Failed to create k8s controller runtime manager")

	lrpReconciler := createLRPReconciler(logger, controllerClient, clientset, eiriniCfg, mgr.GetScheme())
	taskReconciler := createTaskReconciler(logger, controllerClient, clientset, eiriniCfg, mgr.GetScheme())
	podCrashReconciler := createPodCrashReconciler(logger, eiriniCfg.WorkloadsNamespace, controllerClient, clientset)

	err = builder.
		ControllerManagedBy(mgr).
		For(&eiriniv1.LRP{}).
		Owns(&appsv1.StatefulSet{}).
		Complete(lrpReconciler)
	cmdcommons.ExitfIfError(err, "Failed to build LRP reconciler")

	err = builder.
		ControllerManagedBy(mgr).
		For(&eiriniv1.Task{}).
		Owns(&batchv1.Job{}).
		Complete(taskReconciler)
	cmdcommons.ExitfIfError(err, "Failed to build Task reconciler")

	predicates := []predicate.Predicate{reconciler.NewSourceTypeUpdatePredicate("APP")}
	err = builder.
		ControllerManagedBy(mgr).
		For(&corev1.Pod{}, builder.WithPredicates(predicates...)).
		Complete(podCrashReconciler)
	cmdcommons.ExitfIfError(err, "Failed to build Pod Crash reconciler")

	err = mgr.Start(ctrl.SetupSignalHandler())
	cmdcommons.ExitfIfError(err, "Failed to start manager")
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

func createLRPReconciler(
	logger lager.Logger,
	controllerClient runtimeclient.Client,
	clientset kubernetes.Interface,
	eiriniCfg *eirini.Config,
	scheme *runtime.Scheme,
) *reconciler.LRP {
	stDesirer := &k8s.StatefulSetDesirer{
		Pods:                              client.NewPod(clientset, eiriniCfg.WorkloadsNamespace),
		Secrets:                           client.NewSecret(clientset),
		StatefulSets:                      client.NewStatefulSet(clientset, eiriniCfg.WorkloadsNamespace),
		PodDisruptionBudgets:              client.NewPodDisruptionBudget(clientset),
		EventsClient:                      client.NewEvent(clientset),
		StatefulSetToLRPMapper:            k8s.StatefulSetToLRP,
		RegistrySecretName:                eiriniCfg.Properties.RegistrySecretName,
		LivenessProbeCreator:              k8s.CreateLivenessProbe,
		ReadinessProbeCreator:             k8s.CreateReadinessProbe,
		Logger:                            logger.Session("stateful-set-desirer"),
		ApplicationServiceAccount:         eiriniCfg.Properties.ApplicationServiceAccount,
		AllowAutomountServiceAccountToken: eiriniCfg.Properties.UnsafeAllowAutomountServiceAccountToken,
	}

	return reconciler.NewLRP(
		logger,
		controllerClient,
		stDesirer,
		client.NewStatefulSet(clientset, eiriniCfg.WorkloadsNamespace),
		scheme)
}

func createTaskReconciler(
	logger lager.Logger,
	controllerClient runtimeclient.Client,
	clientset kubernetes.Interface,
	eiriniCfg *eirini.Config,
	scheme *runtime.Scheme) *reconciler.Task {
	taskDesirer := k8s.NewTaskDesirer(
		logger,
		client.NewJob(clientset, eiriniCfg.WorkloadsNamespace),
		client.NewSecret(clientset),
		eiriniCfg.Properties.ApplicationServiceAccount,
		eiriniCfg.Properties.RegistrySecretName,
		eiriniCfg.Properties.UnsafeAllowAutomountServiceAccountToken,
	)

	return reconciler.NewTask(logger, controllerClient, taskDesirer, scheme)
}

func createPodCrashReconciler(
	logger lager.Logger,
	workloadsNamespace string,
	controllerClient runtimeclient.Client,
	clientset kubernetes.Interface) *reconciler.PodCrash {
	eventsClient := client.NewEvent(clientset)
	statefulSetClient := client.NewStatefulSet(clientset, workloadsNamespace)
	crashEventGenerator := eirinievent.NewDefaultCrashEventGenerator(eventsClient)

	return reconciler.NewPodCrash(logger, controllerClient, crashEventGenerator, eventsClient, statefulSetClient)
}

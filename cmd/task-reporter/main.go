package main

import (
	"io/ioutil"
	"net/http"
	"os"
	"path/filepath"

	"code.cloudfoundry.org/eirini"
	cmdcommons "code.cloudfoundry.org/eirini/cmd"
	"code.cloudfoundry.org/eirini/k8s"
	"code.cloudfoundry.org/eirini/k8s/client"
	k8stask "code.cloudfoundry.org/eirini/k8s/informers/task"
	"code.cloudfoundry.org/eirini/k8s/reconciler"
	"code.cloudfoundry.org/eirini/util"
	"code.cloudfoundry.org/lager"
	"github.com/jessevdk/go-flags"
	"github.com/pkg/errors"
	"gopkg.in/yaml.v2"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/client-go/kubernetes"
	kscheme "k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/tools/clientcmd"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
)

type options struct {
	ConfigFile string `short:"c" long:"config" description:"Config for running task-reporter"`
}

const defaultCompletionCallbackRetryLimit = 10

func main() {
	var opts options
	_, err := flags.ParseArgs(&opts, os.Args)
	cmdcommons.ExitfIfError(err, "Failed to parse args")

	cfg, err := readConfigFile(opts.ConfigFile)
	cmdcommons.ExitfIfError(err, "Failed to read config file")

	clientset := cmdcommons.CreateKubeClient(cfg.ConfigPath)

	kubeConfig, err := clientcmd.BuildConfigFromFlags("", cfg.ConfigPath)
	cmdcommons.ExitfIfError(err, "Failed to build kubeconfig")

	httpClient, err := createHTTPClient(cfg)
	cmdcommons.ExitfIfError(err, "Failed to create http client")

	taskLogger := lager.NewLogger("task-informer")
	taskLogger.RegisterSink(lager.NewPrettySink(os.Stdout, lager.DEBUG))

	reporter := k8stask.StateReporter{
		Client: httpClient,
		Logger: taskLogger,
	}

	jobsClient := client.NewJob(clientset, cfg.WorkloadsNamespace)
	podUpdater := client.NewPod(clientset, cfg.WorkloadsNamespace)

	completionCallbackRetryLimit := cfg.CompletionCallbackRetryLimit
	if completionCallbackRetryLimit == 0 {
		completionCallbackRetryLimit = defaultCompletionCallbackRetryLimit
	}

	mgrOptions := manager.Options{
		// do not serve prometheus metrics; disabled because port clashes during integration tests
		MetricsBindAddress: "0",
		Scheme:             kscheme.Scheme,
		Logger:             util.NewLagerLogr(taskLogger),
		Namespace:          cfg.WorkloadsNamespace,
	}

	mgr, err := manager.New(kubeConfig, mgrOptions)
	cmdcommons.ExitfIfError(err, "Failed to create k8s controller runtime manager")

	taskReconciler := k8stask.NewReconciler(taskLogger,
		mgr.GetClient(),
		jobsClient,
		podUpdater,
		reporter,
		initTaskDeleter(clientset, cfg.WorkloadsNamespace),
		completionCallbackRetryLimit,
		cfg.TTLSeconds,
	)

	predicates := []predicate.Predicate{reconciler.NewSourceTypeUpdatePredicate("TASK")}
	err = builder.
		ControllerManagedBy(mgr).
		For(&corev1.Pod{}, builder.WithPredicates(predicates...)).
		Complete(taskReconciler)
	cmdcommons.ExitfIfError(err, "Failed to build task reporter reconciler")

	err = mgr.Start(ctrl.SetupSignalHandler())
	cmdcommons.ExitfIfError(err, "Failed to start manager")
}

func initTaskDeleter(clientset kubernetes.Interface, workloadsNamespace string) k8stask.Deleter {
	logger := lager.NewLogger("task-deleter")
	logger.RegisterSink(lager.NewPrettySink(os.Stdout, lager.DEBUG))

	return k8s.NewTaskDeleter(
		logger,
		client.NewJob(clientset, workloadsNamespace),
		client.NewSecret(clientset),
	)
}

func readConfigFile(path string) (eirini.TaskReporterConfig, error) {
	fileBytes, err := ioutil.ReadFile(filepath.Clean(path))
	if err != nil {
		return eirini.TaskReporterConfig{}, errors.Wrap(err, "failed to read file")
	}

	var conf eirini.TaskReporterConfig
	err = yaml.Unmarshal(fileBytes, &conf)

	return conf, errors.Wrap(err, "failed to unmarshal yaml")
}

func createHTTPClient(cfg eirini.TaskReporterConfig) (*http.Client, error) {
	if cfg.CCTLSDisabled {
		return http.DefaultClient, nil
	}

	return util.CreateTLSHTTPClient(
		[]util.CertPaths{
			{
				Crt: cmdcommons.GetExistingFile(cfg.CCCertPath, eirini.CCCrtPath, "CC Cert"),
				Key: cmdcommons.GetExistingFile(cfg.CCKeyPath, eirini.CCKeyPath, "CC Key"),
				Ca:  cmdcommons.GetExistingFile(cfg.CAPath, eirini.CCCAPath, "CC CA"),
			},
		},
	)
}

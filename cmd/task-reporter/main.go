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
	runtimeclient "sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
)

type options struct {
	ConfigFile string `short:"c" long:"config" description:"Config for running task-reporter"`
}

func main() {
	var opts options
	_, err := flags.ParseArgs(&opts, os.Args)
	cmdcommons.ExitIfError(err)

	cfg, err := readConfigFile(opts.ConfigFile)
	cmdcommons.ExitIfError(err)

	clientset := cmdcommons.CreateKubeClient(cfg.ConfigPath)

	kubeConfig, err := clientcmd.BuildConfigFromFlags("", cfg.ConfigPath)
	cmdcommons.ExitIfError(err)

	httpClient, err := createHTTPClient(cfg)
	cmdcommons.ExitIfError(err)

	taskLogger := lager.NewLogger("task-informer")
	taskLogger.RegisterSink(lager.NewPrettySink(os.Stdout, lager.DEBUG))

	reporter := k8stask.StateReporter{
		Client: httpClient,
		Logger: taskLogger,
	}

	controllerClient, err := runtimeclient.New(kubeConfig, runtimeclient.Options{Scheme: kscheme.Scheme})
	cmdcommons.ExitIfError(err)

	jobsClient := client.NewJob(clientset, cfg.Namespace, cfg.EnableMultiNamespaceSupport)

	taskReconciler := k8stask.NewReconciler(taskLogger,
		controllerClient,
		jobsClient,
		reporter,
		initTaskDeleter(clientset, cfg.Namespace, cfg.EnableMultiNamespaceSupport),
	)

	mgrOptions := manager.Options{
		// do not serve prometheus metrics; disabled because port clashes during integration tests
		MetricsBindAddress: "0",
		Scheme:             kscheme.Scheme,
		Logger:             util.NewLagerLogr(taskLogger),
	}

	if !cfg.EnableMultiNamespaceSupport {
		if cfg.Namespace == "" {
			cmdcommons.Exitf("must set namespace in config when enableMultiNamespaceSupport is not set")
		}

		mgrOptions.Namespace = cfg.Namespace
	}

	mgr, err := manager.New(kubeConfig, mgrOptions)
	cmdcommons.ExitIfError(err)

	predicates := []predicate.Predicate{reconciler.NewSourceTypeUpdatePredicate("TASK")}
	err = builder.
		ControllerManagedBy(mgr).
		For(&corev1.Pod{}, builder.WithPredicates(predicates...)).
		Complete(taskReconciler)
	cmdcommons.ExitIfError(err)

	err = mgr.Start(ctrl.SetupSignalHandler())
	cmdcommons.ExitIfError(err)
}

func initTaskDeleter(clientset kubernetes.Interface, namespace string, enableMultiNamespaceSupport bool) k8stask.Deleter {
	logger := lager.NewLogger("task-deleter")
	logger.RegisterSink(lager.NewPrettySink(os.Stdout, lager.DEBUG))

	return k8s.NewTaskDeleter(
		logger,
		client.NewJob(clientset, namespace, enableMultiNamespaceSupport),
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
				Crt: cmdcommons.GetOrDefault(cfg.CCCertPath, eirini.CCCrtPath),
				Key: cmdcommons.GetOrDefault(cfg.CCKeyPath, eirini.CCKeyPath),
				Ca:  cmdcommons.GetOrDefault(cfg.CAPath, eirini.CCCAPath),
			},
		},
	)
}

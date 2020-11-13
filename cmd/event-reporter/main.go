package main

import (
	"crypto/tls"
	"io/ioutil"
	"os"
	"path/filepath"

	"code.cloudfoundry.org/eirini"
	cmdcommons "code.cloudfoundry.org/eirini/cmd"
	"code.cloudfoundry.org/eirini/events"
	k8sclient "code.cloudfoundry.org/eirini/k8s/client"
	k8sevent "code.cloudfoundry.org/eirini/k8s/informers/event"
	"code.cloudfoundry.org/eirini/k8s/reconciler"
	"code.cloudfoundry.org/eirini/util"
	"code.cloudfoundry.org/lager"
	"code.cloudfoundry.org/tlsconfig"
	"code.cloudfoundry.org/tps/cc_client"
	"github.com/jessevdk/go-flags"
	"github.com/pkg/errors"
	"gopkg.in/yaml.v2"
	corev1 "k8s.io/api/core/v1"
	kscheme "k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/tools/clientcmd"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	runtimeclient "sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
)

type options struct {
	ConfigFile string `short:"c" long:"config" description:"Config for running event-reporter"`
}

func main() {
	var opts options
	_, err := flags.ParseArgs(&opts, os.Args)
	cmdcommons.ExitfIfError(err, "Failed to parse args")

	cfg, err := readConfigFile(opts.ConfigFile)
	cmdcommons.ExitfIfError(err, "Failed to read config file")

	clientset := cmdcommons.CreateKubeClient(cfg.ConfigPath)

	kubeConfig, err := clientcmd.BuildConfigFromFlags("", cfg.ConfigPath)
	cmdcommons.ExitfIfError(err, "Failed to build kubeconfig")

	tlsConf := &tls.Config{} // nolint:gosec // No need to check for min version as the empty config is only used when tls is disabled

	if !cfg.CCTLSDisabled {
		tlsConf, err = createTLSConfig(*cfg)
		cmdcommons.ExitfIfError(err, "Failed to create TLS config")
	}

	client := cc_client.NewCcClient(cfg.CcInternalAPI, tlsConf)
	crashReporterLogger := lager.NewLogger("instance-crash-reporter")
	crashReporterLogger.RegisterSink(lager.NewPrettySink(os.Stdout, lager.DEBUG))

	emitter := events.NewCcCrashEmitter(crashReporterLogger, client)

	crashLogger := lager.NewLogger("instance-crash-informer")
	crashLogger.RegisterSink(lager.NewPrettySink(os.Stdout, lager.DEBUG))

	controllerClient, err := runtimeclient.New(kubeConfig, runtimeclient.Options{Scheme: kscheme.Scheme})
	cmdcommons.ExitfIfError(err, "Failed to create k8s runtime client")

	crashReconciler := k8sevent.NewCrashReconciler(
		crashLogger,
		controllerClient,
		k8sevent.NewDefaultCrashEventGenerator(k8sclient.NewEvent(clientset)),
		emitter,
	)

	mgr, err := manager.New(kubeConfig, manager.Options{
		// do not serve prometheus metrics; disabled because port clashes during integration tests
		MetricsBindAddress: "0",
		Namespace:          cfg.WorkloadsNamespace,
		Scheme:             kscheme.Scheme,
		Logger:             util.NewLagerLogr(crashLogger),
	})
	cmdcommons.ExitfIfError(err, "Failed to create k8s controller runtime manager")

	predicates := []predicate.Predicate{reconciler.NewSourceTypeUpdatePredicate("APP")}
	err = builder.
		ControllerManagedBy(mgr).
		For(&corev1.Pod{}, builder.WithPredicates(predicates...)).
		Complete(crashReconciler)
	cmdcommons.ExitfIfError(err, "Failed to build Crash reconciler")

	err = mgr.Start(ctrl.SetupSignalHandler())
	cmdcommons.ExitfIfError(err, "Failed to start manager")
}

func readConfigFile(path string) (*eirini.EventReporterConfig, error) {
	fileBytes, err := ioutil.ReadFile(filepath.Clean(path))
	if err != nil {
		return nil, errors.Wrap(err, "failed to read file")
	}

	var conf eirini.EventReporterConfig
	err = yaml.Unmarshal(fileBytes, &conf)

	return &conf, errors.Wrap(err, "failed to unmarshal yaml")
}

func createTLSConfig(cfg eirini.EventReporterConfig) (*tls.Config, error) {
	crtPath := cmdcommons.GetExistingFile(cfg.CCCertPath, eirini.CCCrtPath, "CC Cert")
	keyPath := cmdcommons.GetExistingFile(cfg.CCKeyPath, eirini.CCKeyPath, "CC Key")
	caPath := cmdcommons.GetExistingFile(cfg.CCCAPath, eirini.CCCAPath, "CC CA")

	return tlsconfig.Build(
		tlsconfig.WithInternalServiceDefaults(),
		tlsconfig.WithIdentityFromFile(crtPath, keyPath),
	).Client(
		tlsconfig.WithAuthorityFromFile(caPath),
	)
}

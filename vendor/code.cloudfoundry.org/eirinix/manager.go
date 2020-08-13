package extension

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"time"

	"code.cloudfoundry.org/quarks-utils/pkg/credsgen"
	inmemorycredgen "code.cloudfoundry.org/quarks-utils/pkg/credsgen/in_memory_generator"
	kubeConfig "code.cloudfoundry.org/quarks-utils/pkg/kubeconfig"
	"code.cloudfoundry.org/eirinix/util/ctxlog"
	"github.com/pkg/errors"
	"github.com/spf13/afero"
	"go.uber.org/zap"
	admissionregistrationv1beta1 "k8s.io/api/admissionregistration/v1beta1"
	corev1 "k8s.io/api/core/v1"
	meta "k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	fields "k8s.io/apimachinery/pkg/fields"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	machinerytypes "k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/watch"
	corev1client "k8s.io/client-go/kubernetes/typed/core/v1"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/cache"
	watchtools "k8s.io/client-go/tools/watch"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/webhook"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"
)

const (
	LabelGUID        = "cloudfoundry.org/guid"
	LabelVersion     = "cloudfoundry.org/version"
	LabelAppGUID     = "cloudfoundry.org/app_guid"
	LabelProcessType = "cloudfoundry.org/process_type"
	LabelSourceType  = "cloudfoundry.org/source_type"
)

// WatcherChannelClosedError can be used to filter for "watcher channel closed"
// in a block like this:
// if err, ok := err.(*extension.WatcherChannelClosedError); ok { // Do things }
type WatcherChannelClosedError struct {
	err string
}

// Error implements the error Interface for WatcherChannelClosedError
func (e *WatcherChannelClosedError) Error() string {
	return e.err
}

// DefaultExtensionManager represent an implementation of Manager
type DefaultExtensionManager struct {
	// Extensions is the list of the Extensions that will be registered by the Manager
	Extensions []Extension

	// Watchers is the list of Eirini watchers handlers
	Watchers []Watcher

	// KubeManager is the kubernetes manager object which is setted up by the Manager
	KubeManager manager.Manager

	// Logger is the logger used internally and accessible to the Extensions
	Logger *zap.SugaredLogger

	// Context is the context structure used by internal components
	Context context.Context

	// WebhookConfig is the webhook configuration used to generate certificates
	WebhookConfig *WebhookConfig

	// WebhookServer is the webhook server where the Manager registers the Extensions to.
	WebhookServer *webhook.Server

	// Credsgen is the credential generator implementation used for generating certificates
	Credsgen credsgen.Generator

	// Options are the manager options
	Options ManagerOptions

	kubeConnection *rest.Config
	kubeClient     corev1client.CoreV1Interface

	stopChannel chan struct{}

	watcher watch.Interface
}

// ManagerOptions represent the Runtime manager options
type ManagerOptions struct {

	// Namespace is the namespace where pods will trigger the extension. Use empty to trigger on all namespaces.
	Namespace string

	// Host is the listening host address for the Manager
	Host string

	// Port is the listening port
	Port int32

	// KubeConfig is the kubeconfig path. Optional, omit for in-cluster connection
	KubeConfig string

	// Logger is the default logger. Optional, if omitted a new one will be created
	Logger *zap.SugaredLogger

	// FailurePolicy default failure policy for the webhook server.  Optional, defaults to fail
	FailurePolicy *admissionregistrationv1beta1.FailurePolicyType

	// FilterEiriniApps enables or disables Eirini apps filters.  Optional, defaults to true
	FilterEiriniApps *bool

	// OperatorFingerprint is a unique string identifiying the Manager.  Optional, defaults to eirini-x
	OperatorFingerprint string

	// SetupCertificateName is the name of the generated certificates.  Optional, defaults uses OperatorFingerprint to generate a new one
	SetupCertificateName string

	// RegisterWebHook enables or disables automatic registering of webhooks. Defaults to true
	RegisterWebHook *bool

	// SetupCertificate enables or disables automatic certificate generation. Defaults to true
	SetupCertificate *bool

	// ServiceName registers the Extension as a MutatingWebhook reachable by a service
	ServiceName string

	// WebhookNamespace, when ServiceName is supplied, a WebhookNamespace is required to indicate in which namespace the webhook service runs on
	WebhookNamespace string

	// WatcherStartRV is the starting ResourceVersion of the PodList which is being watched (see Kubernetes #74022).
	// If omitted, it will start watching from the current RV.
	WatcherStartRV string
}

// Config controls the behaviour of different controllers
type Config struct {
	CtxTimeOut time.Duration

	// Namespace that is being watched by controllers
	Namespace         string
	WebhookServerHost string
	WebhookServerPort int32
	Fs                afero.Fs
}

var addToSchemes = runtime.SchemeBuilder{}

// AddToScheme adds all Resources to the Scheme
func AddToScheme(s *runtime.Scheme) error {
	return addToSchemes.AddToScheme(s)
}

// NewManager returns a manager for the kubernetes cluster.
// the kubeconfig file and the logger are optional
func NewManager(opts ManagerOptions) Manager {

	if opts.Logger == nil {
		z, e := zap.NewProduction()
		if e != nil {
			panic(errors.New("Cannot create logger"))
		}
		defer z.Sync() // flushes buffer, if any
		sugar := z.Sugar()
		opts.Logger = sugar
	}

	if opts.FailurePolicy == nil {
		failurePolicy := admissionregistrationv1beta1.Fail
		opts.FailurePolicy = &failurePolicy
	}

	if len(opts.OperatorFingerprint) == 0 {
		opts.OperatorFingerprint = "eirini-x"
	}

	if len(opts.SetupCertificateName) == 0 {
		opts.SetupCertificateName = opts.getSetupCertificateName()
	}

	if opts.FilterEiriniApps == nil {
		filterEiriniApps := true
		opts.FilterEiriniApps = &filterEiriniApps
	}

	if opts.RegisterWebHook == nil {
		registerWebHook := true
		opts.RegisterWebHook = &registerWebHook
	}

	if opts.SetupCertificate == nil {
		setupCertificate := true
		opts.SetupCertificate = &setupCertificate
	}

	return &DefaultExtensionManager{Options: opts, Logger: opts.Logger, stopChannel: make(chan struct{})}
}

// AddExtension adds an Erini extension to the manager
func (m *DefaultExtensionManager) AddExtension(e Extension) {
	m.Extensions = append(m.Extensions, e)
}

// ListExtensions returns the list of the Extensions added to the Manager
func (m *DefaultExtensionManager) ListExtensions() []Extension {
	return m.Extensions
}

// AddWatcher adds an Erini watcher Extension to the manager
func (m *DefaultExtensionManager) AddWatcher(w Watcher) {
	m.Watchers = append(m.Watchers, w)
}

// ListWatchers returns the list of the Extensions added to the Manager
func (m *DefaultExtensionManager) ListWatchers() []Watcher {
	return m.Watchers
}

// GetKubeClient returns a kubernetes Corev1 client interface from the rest config used.
func (m *DefaultExtensionManager) GetKubeClient() (corev1client.CoreV1Interface, error) {
	if m.kubeClient == nil {
		if m.kubeConnection == nil {
			if _, err := m.GetKubeConnection(); err != nil {
				return nil, err
			}
		}
		client, err := corev1client.NewForConfig(m.kubeConnection)
		if err != nil {
			return nil, errors.Wrap(err, "Could not get kube client")
		}
		m.kubeClient = client
	}

	return m.kubeClient, nil
}

func (m *DefaultExtensionManager) PatchFromPod(req admission.Request, pod *corev1.Pod) admission.Response {
	marshaledPod, err := json.Marshal(pod)
	if err != nil {
		return admission.Errored(http.StatusInternalServerError, err)
	}

	return admission.PatchResponseFromRaw(req.Object.Raw, marshaledPod)
}

// GenWatcher generates a watcher from a corev1client interface
func (m *DefaultExtensionManager) GenWatcher(client corev1client.CoreV1Interface) (watch.Interface, error) {
	podInterface := client.Pods(m.Options.Namespace)

	startResourceVersion := m.Options.WatcherStartRV

	if startResourceVersion == "" {
		lw := cache.NewListWatchFromClient(client.RESTClient(), "pods", m.Options.Namespace, fields.Everything())
		list, err := lw.List(metav1.ListOptions{})
		if err != nil {
			return nil, err
		}

		metaObj, err := meta.ListAccessor(list)
		if err != nil {
			return nil, err
		}

		startResourceVersion = metaObj.GetResourceVersion()
	}

	return watchtools.NewRetryWatcher(startResourceVersion, &cache.ListWatch{
		WatchFunc: func(options metav1.ListOptions) (watch.Interface, error) {
			options.Watch = true

			if m.Options.FilterEiriniApps != nil && *m.Options.FilterEiriniApps {
				options.LabelSelector = LabelSourceType + "=APP"
			}

			return podInterface.Watch(m.Context, options)
		}})
}

// GetLogger returns the Manager injected logger
func (m *DefaultExtensionManager) GetLogger() *zap.SugaredLogger {
	return m.Logger
}

// GetManagerOptions returns the Manager options
func (m *DefaultExtensionManager) GetManagerOptions() ManagerOptions {
	return m.Options
}

func (m *DefaultExtensionManager) kubeSetup() error {
	restConfig, err := kubeConfig.NewGetter(m.Logger).Get(m.Options.KubeConfig)
	if err != nil {
		return err
	}
	if err := kubeConfig.NewChecker(m.Logger).Check(restConfig); err != nil {
		return err
	}
	m.kubeConnection = restConfig

	return nil
}

// GenWebHookServer prepares the webhook server structures
func (m *DefaultExtensionManager) GenWebHookServer() {

	//disableConfigInstaller := true
	m.Context = ctxlog.NewManagerContext(m.Logger)
	m.WebhookConfig = NewWebhookConfig(
		m.KubeManager.GetClient(),
		&Config{
			CtxTimeOut:        10 * time.Second,
			Namespace:         m.Options.Namespace,
			WebhookServerHost: m.Options.Host,
			WebhookServerPort: m.Options.Port,
			Fs:                afero.NewOsFs(),
		},
		m.Credsgen,
		fmt.Sprintf("%s-mutating-hook", m.Options.OperatorFingerprint),
		m.Options.SetupCertificateName,
		m.Options.ServiceName,
		m.Options.WebhookNamespace)

	hookServer := m.KubeManager.GetWebhookServer()
	hookServer.CertDir = m.WebhookConfig.CertDir
	hookServer.Port = int(m.Options.Port)
	hookServer.Host = m.Options.Host
	m.WebhookServer = hookServer
}

// OperatorSetup prepares the webhook server, generates certificates and configuration.
// It also setups the namespace label for the operator
func (m *DefaultExtensionManager) OperatorSetup() error {

	m.GenWebHookServer()

	if m.Options.Namespace != "" {
		if err := m.setOperatorNamespaceLabel(); err != nil {
			return errors.Wrap(err, "setting the operator namespace label")
		}
	}

	if *m.Options.SetupCertificate {
		if err := m.WebhookConfig.setupCertificate(m.Context); err != nil {
			return errors.Wrap(err, "setting up the webhook server certificate")
		}
	}
	return nil
}

func (m *DefaultExtensionManager) setOperatorNamespaceLabel() error {
	c := m.KubeManager.GetClient()
	ctx := m.Context
	ns := &unstructured.Unstructured{}
	ns.SetGroupVersionKind(schema.GroupVersionKind{
		Group:   "",
		Kind:    "Namespace",
		Version: "v1",
	})
	err := c.Get(ctx, machinerytypes.NamespacedName{Name: m.Options.Namespace}, ns)

	if err != nil {
		return errors.Wrap(err, "getting the namespace object")
	}

	labels := ns.GetLabels()
	if labels == nil {
		labels = map[string]string{}
	}
	labels[m.Options.getDefaultNamespaceLabel()] = m.Options.Namespace
	ns.SetLabels(labels)
	err = c.Update(ctx, ns)

	if err != nil {
		return errors.Wrap(err, "updating the namespace object")
	}

	return nil
}

// GetKubeConnection sets up a connection to a Kubernetes cluster if not existing.
func (m *DefaultExtensionManager) GetKubeConnection() (*rest.Config, error) {
	if m.kubeConnection == nil {
		if err := m.kubeSetup(); err != nil {
			return nil, err
		}
	}
	return m.kubeConnection, nil
}

// SetKubeConnection sets a rest config from a given one
func (m *DefaultExtensionManager) SetKubeConnection(c *rest.Config) {
	m.kubeConnection = c
}

// SetKubeClient sets a kube client corev1 from a given one
func (m *DefaultExtensionManager) SetKubeClient(c corev1client.CoreV1Interface) {
	m.kubeClient = c
}

// SetManagerOptions sets the ManagerOptions with the provided one
func (m *DefaultExtensionManager) SetManagerOptions(o ManagerOptions) {
	m.Options = o
}

// RegisterExtensions generates the manager and the operator setup, and loads the extensions to the webhook server
func (m *DefaultExtensionManager) RegisterExtensions() error {
	if err := m.generateManager(); err != nil {
		return err
	}

	if err := m.OperatorSetup(); err != nil {
		return err
	}

	// Setup Scheme for all resources
	if err := AddToScheme(m.KubeManager.GetScheme()); err != nil {
		return err
	}

	return m.LoadExtensions()
}

// LoadExtensions generates and register webhooks from the Extensions added to the Manager
func (m *DefaultExtensionManager) LoadExtensions() error {

	var webhooks []MutatingWebhook
	for k, e := range m.Extensions {
		w := NewWebhook(e, m)
		err := w.RegisterAdmissionWebHook(m.WebhookServer,
			WebhookOptions{
				ID:             strconv.Itoa(k),
				Manager:        m.KubeManager,
				ManagerOptions: m.Options,
			})
		if err != nil {
			return err
		}
		webhooks = append(webhooks, w)
	}

	if m.Options.RegisterWebHook == nil || m.Options.RegisterWebHook != nil && *m.Options.RegisterWebHook {
		if err := m.WebhookConfig.registerWebhooks(m.Context, webhooks); err != nil {
			return errors.Wrap(err, "generating the webhook server configuration")
		}
	}
	return nil
}

func (m *DefaultExtensionManager) generateManager() error {
	m.Credsgen = inmemorycredgen.NewInMemoryGenerator(m.Logger)
	kubeConn, err := m.GetKubeConnection()
	if err != nil {
		return errors.Wrap(err, "Failed connecting to kubernetes cluster")
	}

	mgr, err := manager.New(
		kubeConn,
		manager.Options{
			Namespace:          m.Options.Namespace,
			MetricsBindAddress: "0",
			LeaderElection:     false,
			Port:               int(m.Options.Port),
			Host:               m.Options.Host,
		})
	if err != nil {
		return err
	}

	m.KubeManager = mgr

	return nil
}

// HandleEvent handles a watcher event.
// It propagates the event to all the registered watchers.
func (m *DefaultExtensionManager) HandleEvent(e watch.Event) {
	for _, w := range m.Watchers {
		w.Handle(m, e)
	}
}

// ReadWatcherEvent tries to read events from the watcher channel. It should be run in a loop.
func (m *DefaultExtensionManager) ReadWatcherEvent(w watch.Interface) {
	resultChannel := w.ResultChan()

	for e := range resultChannel {
		m.HandleEvent(e)
	}
}

// Watch starts the Watchers Manager infinite loop, and returns an error on failure
func (m *DefaultExtensionManager) Watch() error {
	defer m.Logger.Sync()

	client, err := m.GetKubeClient()
	if err != nil {
		return err
	}
	watcher, err := m.GenWatcher(client)
	if err != nil {
		return err
	}
	m.Context = ctxlog.NewManagerContext(m.Logger)

	m.watcher = watcher

	m.ReadWatcherEvent(watcher)

	return &WatcherChannelClosedError{"Watcher channel closed"}
}

// Start starts the Manager infinite loop, and returns an error on failure
func (m *DefaultExtensionManager) Start() error {
	defer m.Logger.Sync()

	if err := m.RegisterExtensions(); err != nil {
		return err
	}

	return m.KubeManager.Start(m.stopChannel)
}

func (m *DefaultExtensionManager) Stop() {
	defer m.Logger.Sync()

	close(m.stopChannel)
	if m.watcher != nil {
		m.watcher.Stop()
	}
}

func (o *ManagerOptions) getDefaultNamespaceLabel() string {
	return fmt.Sprintf("%s-ns", o.OperatorFingerprint)
}

func (o *ManagerOptions) getSetupCertificateName() string {
	return fmt.Sprintf("%s-setupcertificate", o.OperatorFingerprint)
}

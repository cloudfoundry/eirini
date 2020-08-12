package extension

import (
	"context"
	"fmt"

	"github.com/pkg/errors"
	admissionregistrationv1beta1 "k8s.io/api/admissionregistration/v1beta1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/webhook"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"
)

type setReferenceFunc func(owner, object metav1.Object, scheme *runtime.Scheme) error

// DefaultMutatingWebhook is the implementation of the Webhook generated out of the Eirini Extension
type DefaultMutatingWebhook struct {
	decoder *admission.Decoder
	client  client.Client

	// EiriniExtension is the Eirini extension associated with the webhook
	EiriniExtension Extension

	// EiriniExtensionManager is the Manager which will be injected into the Handle.
	EiriniExtensionManager Manager

	// FilterEiriniApps indicates if the webhook will filter Eirini apps or not.
	FilterEiriniApps bool
	setReference     setReferenceFunc

	// Name is the name of the webhook
	Name string
	// Path is the path this webhook will serve.
	Path string
	// Rules maps to the Rules field in admissionregistrationv1beta1.Webhook
	Rules []admissionregistrationv1beta1.RuleWithOperations
	// FailurePolicy maps to the FailurePolicy field in admissionregistrationv1beta1.Webhook
	// This optional. If not set, will be defaulted to Ignore (fail-open) by the server.
	// More details: https://github.com/kubernetes/api/blob/f5c295feaba2cbc946f0bbb8b535fc5f6a0345ee/admissionregistration/v1beta1/types.go#L144-L147
	FailurePolicy admissionregistrationv1beta1.FailurePolicyType
	// NamespaceSelector maps to the NamespaceSelector field in admissionregistrationv1beta1.Webhook
	// This optional.
	NamespaceSelector *metav1.LabelSelector
	// Handlers contains a list of handlers. Each handler may only contains the business logic for its own feature.
	// For example, feature foo and bar can be in the same webhook if all the other configurations are the same.
	// The handler will be invoked sequentially as the order in the list.
	// Note: if you are using mutating webhook with multiple handlers, it's your responsibility to
	// ensure the handlers are not generating conflicting JSON patches.
	Handler admission.Handler
	// Webhook contains the Admission webhook information that we register with the controller runtime.
	Webhook *webhook.Admission
}

func (w *DefaultMutatingWebhook) GetName() string {
	return w.Name
}

func (w *DefaultMutatingWebhook) GetRules() []admissionregistrationv1beta1.RuleWithOperations {
	return w.Rules
}

func (w *DefaultMutatingWebhook) GetFailurePolicy() admissionregistrationv1beta1.FailurePolicyType {
	return w.FailurePolicy
}

func (w *DefaultMutatingWebhook) GetNamespaceSelector() *metav1.LabelSelector {
	return w.NamespaceSelector
}

func (w *DefaultMutatingWebhook) GetHandler() admission.Handler {
	return w.Handler
}

func (w *DefaultMutatingWebhook) GetWebhook() *webhook.Admission {
	return w.Webhook
}

func (w *DefaultMutatingWebhook) GetPath() string {
	return w.Path
}

// GetPod retrieves a pod from a types.Request
func (w *DefaultMutatingWebhook) GetPod(req admission.Request) (*corev1.Pod, error) {
	pod := &corev1.Pod{}
	if w.decoder == nil {
		return nil, errors.New("No decoder injected")
	}
	err := w.decoder.Decode(req, pod)
	return pod, err
}

// WebhookOptions are the options required to register a WebHook to the WebHook server
type WebhookOptions struct {
	ID             string // Webhook path will be generated out of that
	MatchLabels    map[string]string
	Manager        manager.Manager
	ManagerOptions ManagerOptions
}

// NewWebhook returns a MutatingWebhook out of an Eirini Extension
func NewWebhook(e Extension, m Manager) MutatingWebhook {
	return &DefaultMutatingWebhook{EiriniExtensionManager: m, EiriniExtension: e, setReference: controllerutil.SetControllerReference}
}

func (w *DefaultMutatingWebhook) getNamespaceSelector(opts WebhookOptions) *metav1.LabelSelector {
	if len(opts.MatchLabels) == 0 {
		return &metav1.LabelSelector{
			MatchLabels: map[string]string{
				opts.ManagerOptions.getDefaultNamespaceLabel(): opts.ManagerOptions.Namespace,
			},
		}
	}
	return &metav1.LabelSelector{MatchLabels: opts.MatchLabels}
}

// RegisterAdmissionWebHook registers the Mutating WebHook to the WebHook Server and returns the generated Admission Webhook
func (w *DefaultMutatingWebhook) RegisterAdmissionWebHook(server *webhook.Server, opts WebhookOptions) error {
	if opts.ManagerOptions.FailurePolicy == nil {
		return errors.New("No failure policy set")
	}
	if opts.ManagerOptions.FilterEiriniApps != nil {
		w.FilterEiriniApps = *opts.ManagerOptions.FilterEiriniApps
	} else {
		w.FilterEiriniApps = true
	}

	globalScopeType := admissionregistrationv1beta1.ScopeType("*")

	w.FailurePolicy = *opts.ManagerOptions.FailurePolicy
	w.Rules = []admissionregistrationv1beta1.RuleWithOperations{
		{
			Rule: admissionregistrationv1beta1.Rule{
				APIGroups:   []string{""},
				APIVersions: []string{"v1"},
				Resources:   []string{"pods"},
				Scope:       &globalScopeType,
			},
			Operations: []admissionregistrationv1beta1.OperationType{
				"CREATE",
				"UPDATE",
			},
		},
	}
	w.Path = fmt.Sprintf("/%s", opts.ID)

	w.Name = fmt.Sprintf("%s.%s.org", opts.ID, opts.ManagerOptions.OperatorFingerprint)
	if opts.ManagerOptions.Namespace != "" {
		w.NamespaceSelector = w.getNamespaceSelector(opts)
	}
	w.Webhook = &admission.Webhook{
		Handler: w,
	}

	if server == nil {
		return errors.New("The Mutating webhook needs a Webhook server to register to")
	}
	server.Register(w.Path, w.Webhook)
	return nil
}

// InjectClient injects the client.
func (w *DefaultMutatingWebhook) InjectClient(c client.Client) error {
	w.client = c
	return nil
}

// InjectDecoder injects the decoder.
func (w *DefaultMutatingWebhook) InjectDecoder(d *admission.Decoder) error {
	w.decoder = d
	return nil
}

// Handle delegates the Handle function to the Eirini Extension
func (w *DefaultMutatingWebhook) Handle(ctx context.Context, req admission.Request) admission.Response {

	pod, _ := w.GetPod(req)

	// Don't filter the pod if We are not handling pods or filtering is disabled
	if pod == nil || !w.FilterEiriniApps {
		return w.EiriniExtension.Handle(ctx, w.EiriniExtensionManager, pod, req)
	}

	podCopy := pod.DeepCopy()

	// Patch only applications pod created by Eirini
	if v, ok := pod.GetLabels()[LabelSourceType]; ok && v == "APP" {
		return w.EiriniExtension.Handle(ctx, w.EiriniExtensionManager, pod, req)
	}

	return w.EiriniExtensionManager.PatchFromPod(req, podCopy)
}

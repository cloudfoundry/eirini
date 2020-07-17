package k8s

import (
	"context"

	"github.com/pkg/errors"

	"code.cloudfoundry.org/eirini"
	"code.cloudfoundry.org/eirini/opi"
	eiriniv1 "code.cloudfoundry.org/eirini/pkg/apis/lrp/v1"
	"github.com/jinzhu/copier"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

type LRPDesirer interface {
	Desire(namespace string, lrp *opi.LRP, opts ...DesirerOption) error
	Get(identifier opi.LRPIdentifier) (*opi.LRP, error)
	Update(lrp *opi.LRP) error
}

func NewLRPReconciler(client client.Client, desirer LRPDesirer, scheme *runtime.Scheme) *LRPReconciler {
	return &LRPReconciler{
		client:  client,
		desirer: desirer,
		scheme:  scheme,
	}
}

type LRPReconciler struct {
	client  client.Client
	desirer LRPDesirer
	scheme  *runtime.Scheme
}

func (r *LRPReconciler) Reconcile(request reconcile.Request) (reconcile.Result, error) {
	lrp := &eiriniv1.LRP{}
	if err := r.client.Get(context.Background(), request.NamespacedName, lrp); err != nil {
		return reconcile.Result{}, err
	}

	err := r.do(lrp)
	return reconcile.Result{}, err
}

func (r *LRPReconciler) do(lrp *eiriniv1.LRP) error {
	_, err := r.desirer.Get(opi.LRPIdentifier{
		GUID:    lrp.Spec.GUID,
		Version: lrp.Spec.Version,
	})
	if errors.Is(err, eirini.ErrNotFound) {
		appLRP, parseErr := toOpiLrp(lrp)
		if err != nil {
			return errors.Wrap(parseErr, "failed to parse the crd spec to the lrp model")
		}
		return r.desirer.Desire(lrp.Namespace, appLRP, r.setOwnerFn(lrp))
	}
	if err != nil {
		return err
	}

	appLRP, err := toOpiLrp(lrp)
	if err != nil {
		return errors.Wrap(err, "failed to parse the crd spec to the lrp model")
	}
	return r.desirer.Update(appLRP)
}

func (r *LRPReconciler) setOwnerFn(lrp *eiriniv1.LRP) func(interface{}) error {
	return func(resource interface{}) error {
		obj := resource.(metav1.Object)
		if err := ctrl.SetControllerReference(lrp, obj, r.scheme); err != nil {
			return err
		}
		return nil
	}
}

func toOpiLrp(lrp *eiriniv1.LRP) (*opi.LRP, error) {
	opiLrp := &opi.LRP{}
	if err := copier.Copy(opiLrp, lrp.Spec); err != nil {
		return nil, err
	}
	opiLrp.TargetInstances = lrp.Spec.Instances
	if err := copier.Copy(&opiLrp.AppURIs, lrp.Spec.AppRoutes); err != nil {
		return nil, err
	}

	return opiLrp, nil
}

package k8s

import (
	"context"
	"errors"

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

func (c *LRPReconciler) Reconcile(request reconcile.Request) (reconcile.Result, error) {
	lrp := &eiriniv1.LRP{}
	err := c.client.Get(context.Background(), request.NamespacedName, lrp)
	if err != nil {
		return reconcile.Result{}, err
	}

	err = c.do(lrp)
	if err != nil {
		return reconcile.Result{}, err
	}

	return reconcile.Result{}, nil
}

func (l *LRPReconciler) do(lrp *eiriniv1.LRP) error {
	_, err := l.desirer.Get(opi.LRPIdentifier{
		GUID:    lrp.Spec.GUID,
		Version: lrp.Spec.Version,
	})
	if errors.Is(err, eirini.ErrNotFound) {
		return l.desirer.Desire(lrp.Namespace, toOpiLrp(lrp), l.setOwnerFn(lrp))
	}
	if err != nil {
		return err
	}

	return l.desirer.Update(toOpiLrp(lrp))
}

func toOpiLrp(lrp *eiriniv1.LRP) *opi.LRP {
	opiLrp := &opi.LRP{}
	copier.Copy(opiLrp, lrp.Spec)
	opiLrp.TargetInstances = lrp.Spec.Instances
	opiLrp.AppURIs = lrp.Spec.AppRoutes
	return opiLrp
}

func (l *LRPReconciler) setOwnerFn(lrp *eiriniv1.LRP) func(interface{}) error {
	return func(resource interface{}) error {
		obj := resource.(metav1.Object)
		if err := ctrl.SetControllerReference(lrp, obj, l.scheme); err != nil {
			return err
		}
		return nil
	}
}

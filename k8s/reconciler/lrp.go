package reconciler

import (
	"context"
	"fmt"

	"code.cloudfoundry.org/eirini"
	"code.cloudfoundry.org/eirini/api"
	"code.cloudfoundry.org/eirini/k8s/shared"
	eiriniv1 "code.cloudfoundry.org/eirini/pkg/apis/eirini/v1"
	"code.cloudfoundry.org/eirini/util"
	"code.cloudfoundry.org/lager"
	"github.com/hashicorp/go-multierror"
	"github.com/jinzhu/copier"
	"github.com/pkg/errors"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

//counterfeiter:generate . LRPWorkloadCLient
//counterfeiter:generate -o reconcilerfakes/fake_controller_runtime_client.go sigs.k8s.io/controller-runtime/pkg/client.Client
//counterfeiter:generate -o reconcilerfakes/fake_status_writer.go sigs.k8s.io/controller-runtime/pkg/client.StatusWriter
//counterfeiter:generate . LRPsCrClient

type LRPWorkloadCLient interface {
	Desire(ctx context.Context, namespace string, lrp *api.LRP, opts ...shared.Option) error
	Get(ctx context.Context, identifier api.LRPIdentifier) (*api.LRP, error)
	Update(ctx context.Context, lrp *api.LRP) error
	GetStatus(ctx context.Context, identifier api.LRPIdentifier) (eiriniv1.LRPStatus, error)
}

type LRPsCrClient interface {
	UpdateLRPStatus(context.Context, *eiriniv1.LRP, eiriniv1.LRPStatus) error
	GetLRP(context.Context, string, string) (*eiriniv1.LRP, error)
}

func NewLRP(logger lager.Logger, lrpsCrClient LRPsCrClient, workloadClient LRPWorkloadCLient, scheme *runtime.Scheme) *LRP {
	return &LRP{
		logger:         logger,
		lrpsCrClient:   lrpsCrClient,
		workloadClient: workloadClient,
		scheme:         scheme,
	}
}

type LRP struct {
	logger         lager.Logger
	lrpsCrClient   LRPsCrClient
	workloadClient LRPWorkloadCLient
	scheme         *runtime.Scheme
}

func (r *LRP) Reconcile(ctx context.Context, request reconcile.Request) (reconcile.Result, error) {
	logger := r.logger.Session("reconcile-lrp",
		lager.Data{
			"name":      request.NamespacedName.Name,
			"namespace": request.NamespacedName.Namespace,
		})

	lrp, err := r.lrpsCrClient.GetLRP(ctx, request.Namespace, request.Name)
	if err != nil {
		if apierrors.IsNotFound(err) {
			logger.Error("lrp-not-found", err)

			return reconcile.Result{}, nil
		}

		logger.Error("failed-to-get-lrp", err)

		return reconcile.Result{}, errors.Wrap(err, "failed to get lrp")
	}

	err = r.do(ctx, lrp)
	if err != nil {
		logger.Error("failed-to-reconcile", err)
	}

	return reconcile.Result{}, err
}

func (r *LRP) do(ctx context.Context, lrp *eiriniv1.LRP) error {
	_, err := r.workloadClient.Get(ctx, api.LRPIdentifier{
		GUID:    lrp.Spec.GUID,
		Version: lrp.Spec.Version,
	})
	if errors.Is(err, eirini.ErrNotFound) {
		appLRP, parseErr := toAPILrp(lrp)
		if parseErr != nil {
			return errors.Wrap(parseErr, "failed to parse the crd spec to the lrp model")
		}

		return errors.Wrap(r.workloadClient.Desire(ctx, lrp.Namespace, appLRP, r.setOwnerFn(lrp)), "failed to desire lrp")
	}

	if err != nil {
		return errors.Wrap(err, "failed to get lrp")
	}

	appLRP, err := toAPILrp(lrp)
	if err != nil {
		return errors.Wrap(err, "failed to parse the crd spec to the lrp model")
	}

	var errs *multierror.Error

	err = r.updateStatus(ctx, lrp)
	errs = multierror.Append(errs, errors.Wrap(err, "failed to update lrp status"))

	err = r.workloadClient.Update(ctx, appLRP)
	errs = multierror.Append(errs, errors.Wrap(err, "failed to update app"))

	return errs.ErrorOrNil()
}

func (r *LRP) updateStatus(ctx context.Context, lrp *eiriniv1.LRP) error {
	lrpStatus, err := r.workloadClient.GetStatus(ctx, api.LRPIdentifier{
		GUID:    lrp.Spec.GUID,
		Version: lrp.Spec.Version,
	})
	if err != nil {
		return err
	}

	return r.lrpsCrClient.UpdateLRPStatus(ctx, lrp, lrpStatus)
}

func (r *LRP) setOwnerFn(lrp *eiriniv1.LRP) func(interface{}) error {
	return func(resource interface{}) error {
		obj, ok := resource.(metav1.Object)
		if !ok {
			return fmt.Errorf("failed to cast %v to metav1.Object", resource)
		}

		if err := ctrl.SetControllerReference(lrp, obj, r.scheme); err != nil {
			return errors.Wrap(err, "failed to set controller reference")
		}

		return nil
	}
}

func toAPILrp(lrp *eiriniv1.LRP) (*api.LRP, error) {
	apiLrp := &api.LRP{}
	if err := copier.Copy(apiLrp, lrp.Spec); err != nil {
		return nil, errors.Wrap(err, "failed to copy lrp spec")
	}

	apiLrp.TargetInstances = lrp.Spec.Instances

	if lrp.Spec.PrivateRegistry != nil {
		apiLrp.PrivateRegistry.Server = util.ParseImageRegistryHost(lrp.Spec.Image)
	}

	return apiLrp, nil
}

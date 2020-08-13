package webhook

import (
	"context"
	"errors"
	"net/http"
	"strconv"

	"code.cloudfoundry.org/eirini"
	"code.cloudfoundry.org/eirini/k8s"
	"code.cloudfoundry.org/eirini/util"
	eirinix "code.cloudfoundry.org/eirinix"
	"code.cloudfoundry.org/lager"
	"k8s.io/api/admission/v1beta1"
	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"
)

//counterfeiter:generate -o webhookfakes/fake_manager.go ../../vendor/code.cloudfoundry.org/eirinix Manager

type InstanceIndexEnvInjector struct {
	logger lager.Logger
}

func NewInstanceIndexEnvInjector(logger lager.Logger) InstanceIndexEnvInjector {
	return InstanceIndexEnvInjector{
		logger: logger,
	}
}

func (i InstanceIndexEnvInjector) Handle(ctx context.Context, eiriniManager eirinix.Manager, pod *corev1.Pod, req admission.Request) admission.Response {
	logger := i.logger.Session("handle-webhook-request")

	if req.Operation != v1beta1.Create {
		return admission.Allowed("pod was already created")
	}

	if pod == nil {
		err := errors.New("no pod could be decoded from the request")
		logger.Error("no-pod-in-request", err)

		return admission.Errored(http.StatusBadRequest, err)
	}

	logger = logger.WithData(lager.Data{"pod-name": pod.Name, "pod-namespace": pod.Namespace})

	podCopy := pod.DeepCopy()

	err := injectInstanceIndex(logger, podCopy)
	if err != nil {
		i.logger.Error("failed-to-inject-instance-index", err)

		return admission.Errored(http.StatusBadRequest, err)
	}

	return eiriniManager.PatchFromPod(req, podCopy)
}

func injectInstanceIndex(logger lager.Logger, pod *corev1.Pod) error {
	index, err := util.ParseAppIndex(pod.Name)
	if err != nil {
		return err
	}

	for c := range pod.Spec.Containers {
		container := &pod.Spec.Containers[c]
		if container.Name == k8s.OPIContainerName {
			cfInstanceVar := corev1.EnvVar{Name: eirini.EnvCFInstanceIndex, Value: strconv.Itoa(index)}
			container.Env = append(container.Env, cfInstanceVar)

			logger.Debug("patching-instance-index", lager.Data{"env-var": cfInstanceVar})

			return nil
		}
	}

	logger.Info("no-opi-container-found")

	return errors.New("no opi container found in pod")
}

package k8s

import (
	"code.cloudfoundry.org/eirini/api"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
)

const (
	livenessFailureThreshold  = 4
	readinessFailureThreshold = 1
)

func CreateLivenessProbe(lrp *api.LRP) *v1.Probe {
	initialDelay := toSeconds(lrp.Health.TimeoutMs)

	if lrp.Health.Type == "http" {
		return createHTTPProbe(lrp, initialDelay, livenessFailureThreshold)
	} else if lrp.Health.Type == "port" {
		return createPortProbe(lrp, initialDelay, livenessFailureThreshold)
	}

	return nil
}

func CreateReadinessProbe(lrp *api.LRP) *v1.Probe {
	if lrp.Health.Type == "http" {
		return createHTTPProbe(lrp, 0, readinessFailureThreshold)
	} else if lrp.Health.Type == "port" {
		return createPortProbe(lrp, 0, readinessFailureThreshold)
	}

	return nil
}

func createPortProbe(lrp *api.LRP, initialDelay, failureThreshold int32) *v1.Probe {
	return &v1.Probe{
		Handler: v1.Handler{
			TCPSocket: tcpSocketAction(lrp),
		},
		InitialDelaySeconds: initialDelay,
		FailureThreshold:    failureThreshold,
	}
}

func createHTTPProbe(lrp *api.LRP, initialDelay, failureThreshold int32) *v1.Probe {
	return &v1.Probe{
		Handler: v1.Handler{
			HTTPGet: httpGetAction(lrp),
		},
		InitialDelaySeconds: initialDelay,
		FailureThreshold:    failureThreshold,
	}
}

func httpGetAction(lrp *api.LRP) *v1.HTTPGetAction {
	return &v1.HTTPGetAction{
		Path: lrp.Health.Endpoint,
		Port: intstr.IntOrString{Type: intstr.Int, IntVal: lrp.Health.Port},
	}
}

func tcpSocketAction(lrp *api.LRP) *v1.TCPSocketAction {
	return &v1.TCPSocketAction{
		Port: intstr.IntOrString{Type: intstr.Int, IntVal: lrp.Health.Port},
	}
}

func toSeconds(millis uint) int32 {
	return int32(millis / 1000) //nolint:gomnd
}

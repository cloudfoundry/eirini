package k8s

import (
	"code.cloudfoundry.org/eirini/opi"
	"k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
)

func CreateLivenessProbe(lrp *opi.LRP) *v1.Probe {
	checkType := lrp.Health.Type
	if checkType == "http" {
		return &v1.Probe{
			Handler: v1.Handler{
				HTTPGet: httpGetAction(lrp),
			},
			TimeoutSeconds: toSeconds(lrp.Health.TimeoutMs),
		}
	} else if checkType == "port" {
		return &v1.Probe{
			Handler: v1.Handler{
				TCPSocket: tcpSocketAction(lrp),
			},
			TimeoutSeconds: toSeconds(lrp.Health.TimeoutMs),
		}
	}

	return nil
}

func toSeconds(millis uint) int32 {
	seconds := millis / 1000
	return int32(seconds)
}

func httpGetAction(lrp *opi.LRP) *v1.HTTPGetAction {
	return &v1.HTTPGetAction{
		Path: lrp.Health.Endpoint,
		Port: intstr.IntOrString{Type: intstr.Int, IntVal: lrp.Health.Port},
	}
}

func tcpSocketAction(lrp *opi.LRP) *v1.TCPSocketAction {
	return &v1.TCPSocketAction{
		Port: intstr.IntOrString{Type: intstr.Int, IntVal: lrp.Health.Port},
	}
}

package sink

import (
	"code.cloudfoundry.org/runtimeschema/cc_messages"
	"github.com/julz/cube/opi"
)

func Convert(msg cc_messages.DesireAppRequestFromCC) opi.LRP {
	if len(msg.ProcessGuid) > 36 {
		msg.ProcessGuid = msg.ProcessGuid[:36]
	}

	if msg.DockerImageUrl == "" {
		msg.DockerImageUrl = dropletToImageURI(msg)
	}

	return opi.LRP{
		Name:            msg.ProcessGuid,
		Image:           msg.DockerImageUrl,
		TargetInstances: msg.NumInstances,
	}
}

func dropletToImageURI(msg cc_messages.DesireAppRequestFromCC) string {
	// hardcoded for now because networking is hard and getting the
	// registry visible is a pain for testing
	return "busybox"
}

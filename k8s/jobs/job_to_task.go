package jobs

import (
	"code.cloudfoundry.org/eirini/opi"
	batch "k8s.io/api/batch/v1"
)

func toTask(job batch.Job) *opi.Task {
	return &opi.Task{
		GUID: job.Labels[LabelGUID],
	}
}

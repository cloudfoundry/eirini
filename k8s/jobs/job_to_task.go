package jobs

import (
	"code.cloudfoundry.org/eirini/api"
	batch "k8s.io/api/batch/v1"
)

func toTask(job batch.Job) *api.Task {
	return &api.Task{
		GUID: job.Labels[LabelGUID],
	}
}

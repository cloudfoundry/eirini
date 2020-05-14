package task

import (
	"fmt"
	"net/http"

	"code.cloudfoundry.org/eirini/k8s"
	"code.cloudfoundry.org/eirini/k8s/utils"
	"code.cloudfoundry.org/lager"
	batchv1 "k8s.io/api/batch/v1"
)

type StateReporter struct {
	EiriniAddress string
	Client        *http.Client
	Logger        lager.Logger
}

func (r StateReporter) Report(job *batchv1.Job) {
	if len(job.Status.Conditions) != 0 {
		taskGUID := job.Labels[k8s.LabelGUID]
		uri := fmt.Sprintf("%s/tasks/%s/completed", r.EiriniAddress, taskGUID)
		if err := utils.Put(r.Client, uri, nil); err != nil {
			r.Logger.Error("cannot send task status response", err, lager.Data{"taskGuid": taskGUID})
			return
		}
	}
}

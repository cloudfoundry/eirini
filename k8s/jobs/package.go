package jobs

import "code.cloudfoundry.org/eirini/k8s/stset"

//go:generate go run github.com/maxbrunsfeld/counterfeiter/v6 -generate

const (
	AnnotationGUID                           = "cloudfoundry.org/guid"
	AnnotationAppName                        = stset.AnnotationAppName
	AnnotationAppID                          = stset.AnnotationAppID
	AnnotationOrgName                        = stset.AnnotationOrgName
	AnnotationOrgGUID                        = stset.AnnotationOrgGUID
	AnnotationSpaceName                      = stset.AnnotationSpaceName
	AnnotationSpaceGUID                      = stset.AnnotationSpaceGUID
	AnnotationCompletionCallback             = "cloudfoundry.org/completion_callback"
	AnnotationOpiTaskContainerName           = "cloudfoundry.org/opi-task-container-name"
	AnnotationOpiTaskCompletionReportCounter = "cloudfoundry.org/task_completion_report_counter"
	AnnotationCCAckedTaskCompletion          = "cloudfoundry.org/cc_acked_task_completion"

	LabelGUID       = stset.LabelGUID
	LabelName       = "cloudfoundry.org/name"
	LabelAppGUID    = stset.LabelAppGUID
	LabelSourceType = stset.LabelSourceType

	LabelTaskCompleted = "cloudfoundry.org/task_completed"
	TaskCompletedTrue  = "true"
)

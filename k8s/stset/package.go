package stset

//go:generate go run github.com/maxbrunsfeld/counterfeiter/v6 -generate

const (
	AppSourceType = "APP"

	AnnotationAppName              = "cloudfoundry.org/application_name"
	AnnotationVersion              = "cloudfoundry.org/version"
	AnnotationAppID                = "cloudfoundry.org/application_id"
	AnnotationSpaceName            = "cloudfoundry.org/space_name"
	AnnotationOrgName              = "cloudfoundry.org/org_name"
	AnnotationOrgGUID              = "cloudfoundry.org/org_guid"
	AnnotationSpaceGUID            = "cloudfoundry.org/space_guid"
	AnnotationLastUpdated          = "cloudfoundry.org/last_updated"
	AnnotationProcessGUID          = "cloudfoundry.org/process_guid"
	AnnotationRegisteredRoutes     = "cloudfoundry.org/routes"
	AnnotationOriginalRequest      = "cloudfoundry.org/original_request"
	AnnotationLastReportedAppCrash = "cloudfoundry.org/last_reported_app_crash"
	AnnotationLastReportedLRPCrash = "cloudfoundry.org/last_reported_lrp_crash"

	LabelGUID        = "cloudfoundry.org/guid"
	LabelOrgGUID     = AnnotationOrgGUID
	LabelOrgName     = AnnotationOrgName
	LabelSpaceGUID   = AnnotationSpaceGUID
	LabelSpaceName   = AnnotationSpaceName
	LabelVersion     = "cloudfoundry.org/version"
	LabelAppGUID     = "cloudfoundry.org/app_guid"
	LabelProcessType = "cloudfoundry.org/process_type"
	LabelSourceType  = "cloudfoundry.org/source_type"

	OPIContainerName = "opi"

	PdbMinAvailableInstances          = 1
	PrivateRegistrySecretGenerateName = "private-registry-"
)

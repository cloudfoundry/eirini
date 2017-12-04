package db

import (
	"code.cloudfoundry.org/bbs/models"
	"code.cloudfoundry.org/lager"
)

//go:generate counterfeiter . DesiredLRPDB

type DesiredLRPDB interface {
	DesiredLRPs(logger lager.Logger, filter models.DesiredLRPFilter) ([]*models.DesiredLRP, error)
	DesiredLRPByProcessGuid(logger lager.Logger, processGuid string) (*models.DesiredLRP, error)

	DesiredLRPSchedulingInfos(logger lager.Logger, filter models.DesiredLRPFilter) ([]*models.DesiredLRPSchedulingInfo, error)

	DesireLRP(logger lager.Logger, desiredLRP *models.DesiredLRP) error
	UpdateDesiredLRP(logger lager.Logger, processGuid string, update *models.DesiredLRPUpdate) (beforeDesiredLRP *models.DesiredLRP, err error)
	RemoveDesiredLRP(logger lager.Logger, processGuid string) error
}

package db

import (
	"code.cloudfoundry.org/bbs/models"
	"code.cloudfoundry.org/lager"
)

//go:generate counterfeiter . SuspectDB

type SuspectDB interface {
	RemoveSuspectActualLRP(lager.Logger, *models.ActualLRPKey) (*models.ActualLRP, error)
}

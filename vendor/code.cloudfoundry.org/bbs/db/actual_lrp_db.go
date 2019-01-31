package db

import (
	"code.cloudfoundry.org/bbs/models"
	"code.cloudfoundry.org/lager"
)

//go:generate counterfeiter . ActualLRPDB

type ActualLRPDB interface {
	ActualLRPs(logger lager.Logger, filter models.ActualLRPFilter) ([]*models.ActualLRP, error)
	CreateUnclaimedActualLRP(logger lager.Logger, key *models.ActualLRPKey) (after *models.ActualLRP, err error)
	UnclaimActualLRP(logger lager.Logger, key *models.ActualLRPKey) (before *models.ActualLRP, after *models.ActualLRP, err error)
	ClaimActualLRP(logger lager.Logger, processGuid string, index int32, instanceKey *models.ActualLRPInstanceKey) (before *models.ActualLRP, after *models.ActualLRP, err error)
	StartActualLRP(logger lager.Logger, key *models.ActualLRPKey, instanceKey *models.ActualLRPInstanceKey, netInfo *models.ActualLRPNetInfo) (before *models.ActualLRP, after *models.ActualLRP, err error)
	CrashActualLRP(logger lager.Logger, key *models.ActualLRPKey, instanceKey *models.ActualLRPInstanceKey, crashReason string) (before *models.ActualLRP, after *models.ActualLRP, shouldRestart bool, err error)
	FailActualLRP(logger lager.Logger, key *models.ActualLRPKey, placementError string) (before *models.ActualLRP, after *models.ActualLRP, err error)
	RemoveActualLRP(logger lager.Logger, processGuid string, index int32, instanceKey *models.ActualLRPInstanceKey) error

	ChangeActualLRPPresence(logger lager.Logger, key *models.ActualLRPKey, from, to models.ActualLRP_Presence) (before *models.ActualLRP, after *models.ActualLRP, err error)

	CountActualLRPsByState(logger lager.Logger) (int, int, int, int, int)
	CountDesiredInstances(logger lager.Logger) int
}

package db

import (
	"code.cloudfoundry.org/bbs/models"
	"code.cloudfoundry.org/lager"
)

//go:generate counterfeiter . LRPDB

type ConvergenceResult struct {
	MissingLRPKeys               []*models.ActualLRPKeyWithSchedulingInfo
	UnstartedLRPKeys             []*models.ActualLRPKeyWithSchedulingInfo
	SuspectKeysWithExistingCells []*models.ActualLRPKey
	SuspectLRPKeysToRetire       []*models.ActualLRPKey
	SuspectRunningKeys           []*models.ActualLRPKey
	SuspectClaimedKeys           []*models.ActualLRPKey
	KeysToRetire                 []*models.ActualLRPKey
	KeysWithMissingCells         []*models.ActualLRPKeyWithSchedulingInfo
	MissingCellIds               []string
	Events                       []models.Event
	InstanceEvents               []models.Event
}

type LRPDB interface {
	ActualLRPDB
	DesiredLRPDB

	ConvergeLRPs(logger lager.Logger, cellSet models.CellSet) ConvergenceResult
}

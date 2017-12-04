package db

import (
	"code.cloudfoundry.org/auctioneer"
	"code.cloudfoundry.org/bbs/models"
	"code.cloudfoundry.org/lager"
)

//go:generate counterfeiter . LRPDB

type LRPDB interface {
	ActualLRPDB
	DesiredLRPDB

	ConvergeLRPs(logger lager.Logger, cellSet models.CellSet) (startRequests []*auctioneer.LRPStartRequest, keysWithMissingCells []*models.ActualLRPKeyWithSchedulingInfo, keysToRetire []*models.ActualLRPKey, events []models.Event)

	// Exposed For Test
	GatherAndPruneLRPs(logger lager.Logger, cellSet models.CellSet) (*models.ConvergenceInput, error)
}

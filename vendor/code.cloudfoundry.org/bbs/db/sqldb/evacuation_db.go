package sqldb

import (
	"reflect"

	"code.cloudfoundry.org/bbs/db/sqldb/helpers"
	"code.cloudfoundry.org/bbs/models"
	"code.cloudfoundry.org/lager"
)

func (db *SQLDB) EvacuateActualLRP(
	logger lager.Logger,
	lrpKey *models.ActualLRPKey,
	instanceKey *models.ActualLRPInstanceKey,
	netInfo *models.ActualLRPNetInfo,
) (*models.ActualLRP, error) {
	logger = logger.Session("evacuate-lrp", lager.Data{"lrp_key": lrpKey, "instance_key": instanceKey, "net_info": netInfo})
	logger.Debug("starting")
	defer logger.Debug("complete")

	var actualLRP *models.ActualLRP

	err := db.transact(logger, func(logger lager.Logger, tx helpers.Tx) error {
		var err error
		processGuid := lrpKey.ProcessGuid
		index := lrpKey.Index

		actualLRP, err = db.fetchActualLRPForUpdate(logger, processGuid, index, models.ActualLRP_Evacuating, tx)
		if err == models.ErrResourceNotFound {
			logger.Debug("creating-evacuating-lrp")
			actualLRP, err = db.createEvacuatingActualLRP(logger, lrpKey, instanceKey, netInfo, tx)
			return err
		}

		if err != nil {
			logger.Error("failed-locking-lrp", err)
			return err
		}

		if actualLRP.ActualLRPKey.Equal(lrpKey) &&
			actualLRP.ActualLRPInstanceKey.Equal(instanceKey) &&
			reflect.DeepEqual(actualLRP.ActualLRPNetInfo, *netInfo) {
			logger.Debug("evacuating-lrp-already-exists")
			return models.ErrResourceExists
		}

		now := db.clock.Now().UnixNano()
		actualLRP.ModificationTag.Increment()
		actualLRP.ActualLRPKey = *lrpKey
		actualLRP.ActualLRPInstanceKey = *instanceKey
		actualLRP.Since = now
		actualLRP.ActualLRPNetInfo = *netInfo
		actualLRP.Presence = models.ActualLRP_Evacuating

		netInfoData, err := db.serializeModel(logger, netInfo)
		if err != nil {
			logger.Error("failed-serializing-net-info", err)
			return err
		}

		_, err = db.update(logger, tx, "actual_lrps",
			helpers.SQLAttributes{
				"domain":                 actualLRP.Domain,
				"instance_guid":          actualLRP.InstanceGuid,
				"cell_id":                actualLRP.CellId,
				"net_info":               netInfoData,
				"state":                  actualLRP.State,
				"since":                  actualLRP.Since,
				"modification_tag_index": actualLRP.ModificationTag.Index,
			},
			"process_guid = ? AND instance_index = ? AND presence = ?",
			actualLRP.ProcessGuid, actualLRP.Index, models.ActualLRP_Evacuating,
		)
		if err != nil {
			logger.Error("failed-update-evacuating-lrp", err)
			return err
		}

		return nil
	})

	return actualLRP, err
}

func (db *SQLDB) RemoveEvacuatingActualLRP(logger lager.Logger, lrpKey *models.ActualLRPKey, instanceKey *models.ActualLRPInstanceKey) error {
	logger = logger.Session("remove-evacuating-lrp", lager.Data{"lrp_key": lrpKey, "instance_key": instanceKey})
	logger.Debug("starting")
	defer logger.Debug("complete")

	return db.transact(logger, func(logger lager.Logger, tx helpers.Tx) error {
		processGuid := lrpKey.ProcessGuid
		index := lrpKey.Index

		lrp, err := db.fetchActualLRPForUpdate(logger, processGuid, index, models.ActualLRP_Evacuating, tx)
		if err == models.ErrResourceNotFound {
			logger.Debug("evacuating-lrp-does-not-exist")
			return nil
		}

		if err != nil {
			logger.Error("failed-fetching-actual-lrp", err)
			return err
		}

		if !lrp.ActualLRPInstanceKey.Equal(instanceKey) {
			logger.Debug("actual-lrp-instance-key-mismatch", lager.Data{"instance_key_param": instanceKey, "instance_key_from_db": lrp.ActualLRPInstanceKey})
			return models.ErrActualLRPCannotBeRemoved
		}

		_, err = db.delete(logger, tx, "actual_lrps",
			"process_guid = ? AND instance_index = ? AND presence = ?",
			processGuid, index, models.ActualLRP_Evacuating,
		)
		if err != nil {
			logger.Error("failed-delete", err)
			return models.ErrActualLRPCannotBeRemoved
		}

		return nil
	})
}

func (db *SQLDB) createEvacuatingActualLRP(logger lager.Logger,
	lrpKey *models.ActualLRPKey,
	instanceKey *models.ActualLRPInstanceKey,
	netInfo *models.ActualLRPNetInfo,
	tx helpers.Tx,
) (*models.ActualLRP, error) {
	netInfoData, err := db.serializeModel(logger, netInfo)
	if err != nil {
		logger.Error("failed-serializing-net-info", err)
		return nil, err
	}

	now := db.clock.Now()
	guid, err := db.guidProvider.NextGUID()
	if err != nil {
		return nil, models.ErrGUIDGeneration
	}

	actualLRP := &models.ActualLRP{
		ActualLRPKey:         *lrpKey,
		ActualLRPInstanceKey: *instanceKey,
		ActualLRPNetInfo:     *netInfo,
		State:                models.ActualLRPStateRunning,
		Since:                now.UnixNano(),
		ModificationTag:      models.ModificationTag{Epoch: guid, Index: 0},
		Presence:             models.ActualLRP_Evacuating,
	}

	sqlAttributes := helpers.SQLAttributes{
		"process_guid":           actualLRP.ProcessGuid,
		"instance_index":         actualLRP.Index,
		"presence":               models.ActualLRP_Evacuating,
		"domain":                 actualLRP.Domain,
		"instance_guid":          actualLRP.InstanceGuid,
		"cell_id":                actualLRP.CellId,
		"state":                  actualLRP.State,
		"net_info":               netInfoData,
		"since":                  actualLRP.Since,
		"modification_tag_epoch": actualLRP.ModificationTag.Epoch,
		"modification_tag_index": actualLRP.ModificationTag.Index,
	}

	_, err = db.upsert(logger, tx, "actual_lrps",
		sqlAttributes,
		"process_guid = ? AND instance_index = ? AND presence = ?",
		actualLRP.ProcessGuid, actualLRP.Index, models.ActualLRP_Evacuating,
	)
	if err != nil {
		logger.Error("failed-inserting-evacuating-lrp", err)
		return nil, err
	}

	return actualLRP, nil
}

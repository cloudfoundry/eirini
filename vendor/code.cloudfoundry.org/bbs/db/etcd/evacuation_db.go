package etcd

import (
	"reflect"

	"code.cloudfoundry.org/bbs/models"
	"code.cloudfoundry.org/lager"
)

func (db *ETCDDB) EvacuateActualLRP(
	logger lager.Logger,
	lrpKey *models.ActualLRPKey,
	instanceKey *models.ActualLRPInstanceKey,
	netInfo *models.ActualLRPNetInfo,
	ttl uint64,
) (*models.ActualLRPGroup, error) {
	logger = logger.Session("evacuate-actual-lrp", lager.Data{"process_guid": lrpKey.ProcessGuid, "index": lrpKey.Index})

	logger.Debug("starting")
	defer logger.Debug("complete")

	node, err := db.fetchRaw(logger, EvacuatingActualLRPSchemaPath(lrpKey.ProcessGuid, lrpKey.Index))
	bbsErr := models.ConvertError(err)
	if bbsErr != nil {
		if bbsErr.Type == models.Error_ResourceNotFound {
			return db.createEvacuatingActualLRP(logger, lrpKey, instanceKey, netInfo, ttl)
		}
		return nil, bbsErr
	}

	lrp := models.ActualLRP{}
	err = db.deserializeModel(logger, node, &lrp)
	if err != nil {
		return nil, err
	}

	if lrp.ActualLRPKey.Equal(lrpKey) &&
		lrp.ActualLRPInstanceKey.Equal(instanceKey) &&
		reflect.DeepEqual(lrp.ActualLRPNetInfo, *netInfo) {
		return &models.ActualLRPGroup{Evacuating: &lrp}, nil
	}

	lrp.ActualLRPNetInfo = *netInfo
	lrp.ActualLRPKey = *lrpKey
	lrp.ActualLRPInstanceKey = *instanceKey
	lrp.Since = db.clock.Now().UnixNano()
	lrp.ModificationTag.Increment()

	data, err := db.serializeModel(logger, &lrp)
	if err != nil {
		logger.Error("failed-serializing", err)
		return nil, err
	}

	_, err = db.client.CompareAndSwap(EvacuatingActualLRPSchemaPath(lrp.ProcessGuid, lrp.Index), data, ttl, node.ModifiedIndex)
	if err != nil {
		return nil, ErrorFromEtcdError(logger, err)
	}

	return &models.ActualLRPGroup{Evacuating: &lrp}, nil
}

func (db *ETCDDB) createEvacuatingActualLRP(logger lager.Logger, key *models.ActualLRPKey, instanceKey *models.ActualLRPInstanceKey, netInfo *models.ActualLRPNetInfo, evacuatingTTLInSeconds uint64) (*models.ActualLRPGroup, error) {
	logger.Debug("create-evacuating-actual-lrp")
	defer logger.Debug("create-evacuating-actual-lrp-complete")

	lrp, err := db.newRunningActualLRP(key, instanceKey, netInfo)
	if err != nil {
		return nil, models.ErrActualLRPCannotBeEvacuated
	}

	lrp.ModificationTag.Increment()

	lrpData, err := db.serializeModel(logger, lrp)
	if err != nil {
		return nil, err
	}

	_, err = db.client.Create(EvacuatingActualLRPSchemaPath(key.ProcessGuid, key.Index), lrpData, evacuatingTTLInSeconds)
	if err != nil {
		logger.Error("failed", err)
		return nil, models.ErrActualLRPCannotBeEvacuated
	}

	return &models.ActualLRPGroup{Evacuating: lrp}, nil
}

func (db *ETCDDB) RemoveEvacuatingActualLRP(logger lager.Logger, key *models.ActualLRPKey, instanceKey *models.ActualLRPInstanceKey) error {
	processGuid := key.ProcessGuid
	index := key.Index

	logger = logger.Session("remove-evacuating", lager.Data{"process_guid": processGuid, "index": index})

	logger.Debug("starting")
	defer logger.Debug("complete")

	node, err := db.fetchRaw(logger, EvacuatingActualLRPSchemaPath(processGuid, index))
	bbsErr := models.ConvertError(err)
	if bbsErr != nil {
		if bbsErr.Type == models.Error_ResourceNotFound {
			logger.Debug("evacuating-actual-lrp-not-found")
			return nil
		}
		return bbsErr
	}

	lrp := models.ActualLRP{}
	err = db.deserializeModel(logger, node, &lrp)
	if err != nil {
		return err
	}

	if !lrp.ActualLRPKey.Equal(key) || !lrp.ActualLRPInstanceKey.Equal(instanceKey) {
		return models.ErrActualLRPCannotBeRemoved
	}

	_, err = db.client.CompareAndDelete(EvacuatingActualLRPSchemaPath(lrp.ProcessGuid, lrp.Index), node.ModifiedIndex)
	if err != nil {
		logger.Error("failed-compare-and-delete", err)
		return models.ErrActualLRPCannotBeRemoved
	}

	return nil
}

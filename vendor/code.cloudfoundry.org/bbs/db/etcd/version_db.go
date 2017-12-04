package etcd

import (
	"encoding/json"

	"code.cloudfoundry.org/bbs/models"
	"code.cloudfoundry.org/lager"
)

func (db *ETCDDB) SetVersion(logger lager.Logger, version *models.Version) error {
	logger.Debug("set-version", lager.Data{"version": version})
	defer logger.Debug("set-version-finished")

	value, err := json.Marshal(version)
	if err != nil {
		return err
	}

	_, err = db.client.Set(VersionKey, value, NO_TTL)
	return err
}

func (db *ETCDDB) Version(logger lager.Logger) (*models.Version, error) {
	logger.Debug("get-version")
	defer logger.Debug("get-version-finished")

	node, err := db.fetchRaw(logger, VersionKey)
	if err != nil {
		return nil, err
	}

	var version models.Version
	err = json.Unmarshal([]byte(node.Value), &version)
	if err != nil {
		return nil, models.ErrDeserialize
	}

	return &version, nil
}

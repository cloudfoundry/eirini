package etcd

import (
	"code.cloudfoundry.org/bbs/format"
	"code.cloudfoundry.org/bbs/models"
	"code.cloudfoundry.org/lager"
	"github.com/coreos/go-etcd/etcd"
)

func (db *ETCDDB) SetEncryptionKeyLabel(logger lager.Logger, keyLabel string) error {
	logger.Debug("set-encryption-key-label", lager.Data{"encryption_key_label": keyLabel})
	defer logger.Debug("set-encryption-key-label-finished")

	_, err := db.client.Set(EncryptionKeyLabelKey, []byte(keyLabel), NO_TTL)
	return err
}

func (db *ETCDDB) EncryptionKeyLabel(logger lager.Logger) (string, error) {
	logger.Debug("get-encryption-key-label")
	defer logger.Debug("get-encryption-key-label-finished")

	node, err := db.fetchRaw(logger, EncryptionKeyLabelKey)
	if err != nil {
		return "", err
	}

	return node.Value, nil
}

func (db *ETCDDB) PerformEncryption(logger lager.Logger) error {
	response, err := db.client.Get(V1SchemaRoot, false, true)
	if err != nil {
		err = ErrorFromEtcdError(logger, err)

		// Continue if the root node does not exist
		if err != models.ErrResourceNotFound {
			return err
		}
	}

	if response != nil {
		rootNode := response.Node
		return db.rewriteNode(logger, rootNode)
	}

	return nil
}

func (db *ETCDDB) rewriteNode(logger lager.Logger, node *etcd.Node) error {
	if !node.Dir {
		encoder := format.NewEncoder(db.cryptor)
		payload, err := encoder.Decode([]byte(node.Value))
		if err != nil {
			logger.Error("failed-to-read-node", err, lager.Data{"etcd_key": node.Key})
			return nil
		}
		encryptedPayload, err := encoder.Encode(format.BASE64_ENCRYPTED, payload)
		if err != nil {
			return err
		}
		_, err = db.client.CompareAndSwap(node.Key, encryptedPayload, NO_TTL, node.ModifiedIndex)
		if err != nil {
			logger.Info("failed-to-compare-and-swap", lager.Data{"err": err, "etcd_key": node.Key})
			return nil
		}
	} else {
		for _, child := range node.Nodes {
			err := db.rewriteNode(logger, child)
			if err != nil {
				return err
			}
		}
	}
	return nil
}

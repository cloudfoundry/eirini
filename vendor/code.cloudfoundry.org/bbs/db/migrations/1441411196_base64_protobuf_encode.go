package migrations

import (
	"database/sql"
	"errors"

	"code.cloudfoundry.org/bbs/db/deprecations"
	"code.cloudfoundry.org/bbs/db/etcd"
	"code.cloudfoundry.org/bbs/encryption"
	"code.cloudfoundry.org/bbs/format"
	"code.cloudfoundry.org/bbs/migration"
	"code.cloudfoundry.org/bbs/models"
	"code.cloudfoundry.org/clock"
	"code.cloudfoundry.org/lager"
	goetcd "github.com/coreos/go-etcd/etcd"
)

func init() {
	AppendMigration(NewBase64ProtobufEncode())
}

type Base64ProtobufEncode struct {
	serializer  format.Serializer
	storeClient etcd.StoreClient
}

func NewBase64ProtobufEncode() migration.Migration {
	return &Base64ProtobufEncode{}
}

func (b *Base64ProtobufEncode) Version() int64 {
	return 1441411196
}

func (b *Base64ProtobufEncode) SetStoreClient(storeClient etcd.StoreClient) {
	b.storeClient = storeClient
}

func (b *Base64ProtobufEncode) SetCryptor(cryptor encryption.Cryptor) {
	b.serializer = format.NewSerializer(cryptor)
}

func (b *Base64ProtobufEncode) RequiresSQL() bool {
	return false
}

func (b *Base64ProtobufEncode) SetRawSQLDB(*sql.DB)  {}
func (b *Base64ProtobufEncode) SetClock(clock.Clock) {}
func (b *Base64ProtobufEncode) SetDBFlavor(string)   {}

func (b *Base64ProtobufEncode) Up(logger lager.Logger) error {
	// Desired LRPs
	response, err := b.storeClient.Get(deprecations.DesiredLRPSchemaRoot, false, true)
	if err != nil {
		err = etcd.ErrorFromEtcdError(logger, err)

		// Continue if the root node does not exist
		if err != models.ErrResourceNotFound {
			return err
		}
	}

	if response != nil {
		desiredLRPRootNode := response.Node
		for _, node := range desiredLRPRootNode.Nodes {
			var desiredLRP models.DesiredLRP
			err := b.reWriteNode(logger, node, &desiredLRP)
			if err != nil {
				return err
			}
		}
	}

	// Actual LRPs
	response, err = b.storeClient.Get(etcd.ActualLRPSchemaRoot, false, true)
	if err != nil {
		err = etcd.ErrorFromEtcdError(logger, err)

		// Continue if the root node does not exist
		if err != models.ErrResourceNotFound {
			return err
		}
	}

	if response != nil {
		actualLRPRootNode := response.Node
		for _, processNode := range actualLRPRootNode.Nodes {
			for _, groupNode := range processNode.Nodes {
				for _, actualLRPNode := range groupNode.Nodes {
					var actualLRP models.ActualLRP
					err := b.reWriteNode(logger, actualLRPNode, &actualLRP)
					if err != nil {
						return err
					}
				}
			}
		}
	}

	// Tasks
	response, err = b.storeClient.Get(etcd.TaskSchemaRoot, false, true)
	if err != nil {
		err = etcd.ErrorFromEtcdError(logger, err)

		// Continue if the root node does not exist
		if err != models.ErrResourceNotFound {
			return err
		}
	}

	if response != nil {
		taskRootNode := response.Node
		for _, node := range taskRootNode.Nodes {
			var task models.Task
			err := b.reWriteNode(logger, node, &task)
			if err != nil {
				return err
			}
		}
	}

	return nil
}

func (b *Base64ProtobufEncode) Down(logger lager.Logger) error {
	return errors.New("not implemented")
}

func (b *Base64ProtobufEncode) reWriteNode(logger lager.Logger, node *goetcd.Node, model format.Versioner) error {
	err := b.serializer.Unmarshal(logger, []byte(node.Value), model)
	if err != nil {
		return err
	}

	value, err := b.serializer.Marshal(logger, format.ENCODED_PROTO, model)
	if err != nil {
		return err
	}

	_, err = b.storeClient.CompareAndSwap(node.Key, value, etcd.NO_TTL, node.ModifiedIndex)
	if err != nil {
		return err
	}

	return nil
}

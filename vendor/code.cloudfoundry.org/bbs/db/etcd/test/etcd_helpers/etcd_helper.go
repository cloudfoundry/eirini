package etcd_helpers

import (
	"code.cloudfoundry.org/bbs/db/etcd"
	"code.cloudfoundry.org/bbs/encryption"
	"code.cloudfoundry.org/bbs/format"
	"code.cloudfoundry.org/clock"
	"code.cloudfoundry.org/lager"
	"code.cloudfoundry.org/lager/lagertest"
)

type ETCDHelper struct {
	client     etcd.StoreClient
	format     *format.Format
	serializer format.Serializer
	logger     lager.Logger
	clock      clock.Clock
}

func NewETCDHelper(serializationFormat *format.Format, cryptor encryption.Cryptor, client etcd.StoreClient, clock clock.Clock) *ETCDHelper {
	logger := lagertest.NewTestLogger("etcd-helper")

	return &ETCDHelper{
		client:     client,
		format:     serializationFormat,
		serializer: format.NewSerializer(cryptor),
		logger:     logger,
		clock:      clock,
	}
}

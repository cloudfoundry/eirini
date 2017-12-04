package etcd_helpers

import (
	"code.cloudfoundry.org/bbs/db/etcd"
	"code.cloudfoundry.org/bbs/models"
	etcdclient "github.com/coreos/go-etcd/etcd"

	. "github.com/onsi/gomega"
)

func (t *ETCDHelper) GetInstanceActualLRP(lrpKey *models.ActualLRPKey) (*models.ActualLRP, error) {
	resp, err := t.client.Get(etcd.ActualLRPSchemaPath(lrpKey.ProcessGuid, lrpKey.Index), false, false)
	if etcdErr, ok := err.(*etcdclient.EtcdError); ok && etcdErr.ErrorCode == etcd.ETCDErrKeyNotFound {
		return &models.ActualLRP{}, models.ErrResourceNotFound
	}

	Expect(err).NotTo(HaveOccurred())

	var lrp models.ActualLRP
	err = t.serializer.Unmarshal(t.logger, []byte(resp.Node.Value), &lrp)
	Expect(err).NotTo(HaveOccurred())

	return &lrp, nil
}

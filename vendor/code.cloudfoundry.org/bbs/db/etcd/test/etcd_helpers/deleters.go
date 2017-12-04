package etcd_helpers

import (
	"code.cloudfoundry.org/bbs/db/etcd"
	. "github.com/onsi/gomega"
)

func (t *ETCDHelper) DeleteDesiredLRP(guid string) {
	_, err := t.client.Delete(etcd.DesiredLRPSchedulingInfoSchemaPath(guid), false)
	Expect(err).NotTo(HaveOccurred())
	_, err = t.client.Delete(etcd.DesiredLRPRunInfoSchemaPath(guid), false)
	Expect(err).NotTo(HaveOccurred())
}

func (t *ETCDHelper) DeleteActualLRP(guid string, index int32) {
	key := etcd.ActualLRPSchemaPath(guid, index)
	_, err := t.client.Delete(key, false)
	Expect(err).NotTo(HaveOccurred())
}

func (t *ETCDHelper) DeleteTask(guid string) {
	key := etcd.TaskSchemaPathByGuid(guid)
	_, err := t.client.Delete(key, false)
	Expect(err).NotTo(HaveOccurred())
}

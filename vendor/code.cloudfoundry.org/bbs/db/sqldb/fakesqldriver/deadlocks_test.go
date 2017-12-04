package fakesqldriver_test

import (
	"database/sql/driver"
	"strings"

	"code.cloudfoundry.org/bbs/db/sqldb/fakesqldriver/fakesqldriverfakes"
	"code.cloudfoundry.org/bbs/models"
	"github.com/go-sql-driver/mysql"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Deadlocks", func() {
	BeforeEach(func() {
		fakeConn.PrepareStub = func(query string) (driver.Stmt, error) {
			fakeStmt := &fakesqldriverfakes.FakeStmt{}
			fakeStmt.NumInputReturns(strings.Count(query, "?"))
			fakeStmt.ExecReturns(nil, &mysql.MySQLError{Number: 1213})
			fakeStmt.QueryReturns(nil, &mysql.MySQLError{Number: 1213})
			return fakeStmt, nil
		}
	})

	Context("Domains", func() {
		It("retries on deadlocks", func() {
			_, err := sqlDB.Domains(logger)
			Expect(err).To(HaveOccurred())
			Expect(fakeConn.BeginCallCount()).To(Equal(3))
		})
	})

	Context("UpsertDomain", func() {
		It("retries on deadlocks", func() {
			err := sqlDB.UpsertDomain(logger, "", 0)
			Expect(err).To(HaveOccurred())
			Expect(fakeConn.BeginCallCount()).To(Equal(3))
		})
	})

	Context("EncryptionKeyLabel", func() {
		It("retries on deadlocks", func() {
			_, err := sqlDB.EncryptionKeyLabel(logger)
			Expect(err).To(HaveOccurred())
			Expect(fakeConn.BeginCallCount()).To(Equal(3))
		})
	})

	Context("SetEncryptionKeyLabel", func() {
		It("retries on deadlocks", func() {
			err := sqlDB.SetEncryptionKeyLabel(logger, "")
			Expect(err).To(HaveOccurred())
			Expect(fakeConn.BeginCallCount()).To(Equal(3))
		})
	})

	Context("RemoveEvacuatingActualLRP", func() {
		It("retries on deadlocks", func() {
			err := sqlDB.RemoveEvacuatingActualLRP(logger, &models.ActualLRPKey{}, nil)
			Expect(err).To(HaveOccurred())
			Expect(fakeConn.BeginCallCount()).To(Equal(3))
		})
	})

	Context("DesireTask", func() {
		It("retries on deadlocks", func() {
			_, err := sqlDB.DesireTask(logger, &models.TaskDefinition{}, "", "")
			Expect(err).To(HaveOccurred())
			Expect(fakeConn.BeginCallCount()).To(Equal(3))
		})
	})

	Context("ActualLRPGroupByProcessGuidAndIndex", func() {
		It("retries on deadlocks", func() {
			_, err := sqlDB.ActualLRPGroupByProcessGuidAndIndex(logger, "", 0)
			Expect(err).To(HaveOccurred())
			Expect(fakeConn.BeginCallCount()).To(Equal(3))
		})
	})

	Context("ActualLRPGroups", func() {
		It("retries on deadlocks", func() {
			_, err := sqlDB.ActualLRPGroups(logger, models.ActualLRPFilter{})
			Expect(err).To(HaveOccurred())
			Expect(fakeConn.BeginCallCount()).To(Equal(3))
		})
	})

	Context("ActualLRPGroupsByProcessGuid", func() {
		It("retries on deadlocks", func() {
			_, err := sqlDB.ActualLRPGroupsByProcessGuid(logger, "")
			Expect(err).To(HaveOccurred())
			Expect(fakeConn.BeginCallCount()).To(Equal(3))
		})
	})

	Context("CancelTask", func() {
		It("retries on deadlocks", func() {
			_, _, _, err := sqlDB.CancelTask(logger, "")
			Expect(err).To(HaveOccurred())
			Expect(fakeConn.BeginCallCount()).To(Equal(3))
		})
	})

	Context("ClaimActualLRP", func() {
		It("retries on deadlocks", func() {
			_, _, err := sqlDB.ClaimActualLRP(logger, "", 0, &models.ActualLRPInstanceKey{})
			Expect(err).To(HaveOccurred())
			Expect(fakeConn.BeginCallCount()).To(Equal(3))
		})
	})

	Context("CompleteTask", func() {
		It("retries on deadlocks", func() {
			_, _, err := sqlDB.CompleteTask(logger, "", "", true, "", "")
			Expect(err).To(HaveOccurred())
			Expect(fakeConn.BeginCallCount()).To(Equal(3))
		})
	})

	Context("CrashActualLRP", func() {
		It("retries on deadlocks", func() {
			_, _, _, err := sqlDB.CrashActualLRP(logger, &models.ActualLRPKey{}, &models.ActualLRPInstanceKey{}, "")
			Expect(err).To(HaveOccurred())
			Expect(fakeConn.BeginCallCount()).To(Equal(3))
		})
	})

	Context("CreateUnclaimedActualLRP", func() {
		It("retries on deadlocks", func() {
			_, err := sqlDB.CreateUnclaimedActualLRP(logger, &models.ActualLRPKey{})
			Expect(err).To(HaveOccurred())
			Expect(fakeConn.BeginCallCount()).To(Equal(3))
		})
	})

	Context("DeleteTask", func() {
		It("retries on deadlocks", func() {
			_, err := sqlDB.DeleteTask(logger, "")
			Expect(err).To(HaveOccurred())
			Expect(fakeConn.BeginCallCount()).To(Equal(3))
		})
	})

	Context("DesireLRP", func() {
		It("retries on deadlocks", func() {
			err := sqlDB.DesireLRP(logger, &models.DesiredLRP{})
			Expect(err).To(HaveOccurred())
			Expect(fakeConn.BeginCallCount()).To(Equal(3))
		})
	})

	Context("DesiredLRPByProcessGuid", func() {
		It("retries on deadlocks", func() {
			_, err := sqlDB.DesiredLRPByProcessGuid(logger, "")
			Expect(err).To(HaveOccurred())
			Expect(fakeConn.BeginCallCount()).To(Equal(3))
		})
	})

	Context("DesiredLRPSchedulingInfos", func() {
		It("retries on deadlocks", func() {
			_, err := sqlDB.DesiredLRPSchedulingInfos(logger, models.DesiredLRPFilter{})
			Expect(err).To(HaveOccurred())
			Expect(fakeConn.BeginCallCount()).To(Equal(3))
		})
	})

	Context("DesiredLRPs", func() {
		It("retries on deadlocks", func() {
			_, err := sqlDB.DesiredLRPs(logger, models.DesiredLRPFilter{})
			Expect(err).To(HaveOccurred())
			Expect(fakeConn.BeginCallCount()).To(Equal(3))
		})
	})

	Context("EvacuateActualLRP", func() {
		It("retries on deadlocks", func() {
			_, err := sqlDB.EvacuateActualLRP(logger, &models.ActualLRPKey{}, &models.ActualLRPInstanceKey{}, &models.ActualLRPNetInfo{}, 0)
			Expect(err).To(HaveOccurred())
			Expect(fakeConn.BeginCallCount()).To(Equal(3))
		})
	})

	Context("FailActualLRP", func() {
		It("retries on deadlocks", func() {
			_, _, err := sqlDB.FailActualLRP(logger, &models.ActualLRPKey{}, "")
			Expect(err).To(HaveOccurred())
			Expect(fakeConn.BeginCallCount()).To(Equal(3))
		})
	})

	Context("FailTask", func() {
		It("retries on deadlocks", func() {
			_, _, err := sqlDB.FailTask(logger, "", "")
			Expect(err).To(HaveOccurred())
			Expect(fakeConn.BeginCallCount()).To(Equal(3))
		})
	})

	Context("RemoveActualLRP", func() {
		It("retries on deadlocks", func() {
			err := sqlDB.RemoveActualLRP(logger, "", 0, &models.ActualLRPInstanceKey{})
			Expect(err).To(HaveOccurred())
			Expect(fakeConn.BeginCallCount()).To(Equal(3))
		})
	})

	Context("RemoveDesiredLRP", func() {
		It("retries on deadlocks", func() {
			err := sqlDB.RemoveDesiredLRP(logger, "")
			Expect(err).To(HaveOccurred())
			Expect(fakeConn.BeginCallCount()).To(Equal(3))
		})
	})

	Context("ResolvingTask", func() {
		It("retries on deadlocks", func() {
			_, _, err := sqlDB.ResolvingTask(logger, "")
			Expect(err).To(HaveOccurred())
			Expect(fakeConn.BeginCallCount()).To(Equal(3))
		})
	})

	Context("SetVersion", func() {
		It("retries on deadlocks", func() {
			err := sqlDB.SetVersion(logger, &models.Version{})
			Expect(err).To(HaveOccurred())
			Expect(fakeConn.BeginCallCount()).To(Equal(3))
		})
	})

	Context("StartActualLRP", func() {
		It("retries on deadlocks", func() {
			_, _, err := sqlDB.StartActualLRP(logger, &models.ActualLRPKey{}, &models.ActualLRPInstanceKey{}, &models.ActualLRPNetInfo{})
			Expect(err).To(HaveOccurred())
			Expect(fakeConn.BeginCallCount()).To(Equal(3))
		})
	})

	Context("StartTask", func() {
		It("retries on deadlocks", func() {
			_, _, _, err := sqlDB.StartTask(logger, "", "")
			Expect(err).To(HaveOccurred())
			Expect(fakeConn.BeginCallCount()).To(Equal(3))
		})
	})

	Context("TaskByGuid", func() {
		It("retries on deadlocks", func() {
			_, err := sqlDB.TaskByGuid(logger, "")
			Expect(err).To(HaveOccurred())
			Expect(fakeConn.BeginCallCount()).To(Equal(3))
		})
	})

	Context("Tasks", func() {
		It("retries on deadlocks", func() {
			_, err := sqlDB.Tasks(logger, models.TaskFilter{})
			Expect(err).To(HaveOccurred())
			Expect(fakeConn.BeginCallCount()).To(Equal(3))
		})
	})

	Context("UnclaimActualLRP", func() {
		It("retries on deadlocks", func() {
			_, _, err := sqlDB.UnclaimActualLRP(logger, &models.ActualLRPKey{})
			Expect(err).To(HaveOccurred())
			Expect(fakeConn.BeginCallCount()).To(Equal(3))
		})
	})

	Context("UpdateDesiredLRP", func() {
		It("retries on deadlocks", func() {
			_, err := sqlDB.UpdateDesiredLRP(logger, "", &models.DesiredLRPUpdate{})
			Expect(err).To(HaveOccurred())
			Expect(fakeConn.BeginCallCount()).To(Equal(3))
		})
	})

	Context("Version", func() {
		It("retries on deadlocks", func() {
			_, err := sqlDB.Version(logger)
			Expect(err).To(HaveOccurred())
			Expect(fakeConn.BeginCallCount()).To(Equal(3))
		})
	})
})

package migrations_test

import (
	"crypto/rand"
	"encoding/json"
	"time"

	"code.cloudfoundry.org/bbs/db/deprecations"
	"code.cloudfoundry.org/bbs/db/etcd"
	"code.cloudfoundry.org/bbs/db/migrations"
	"code.cloudfoundry.org/bbs/encryption"
	"code.cloudfoundry.org/bbs/format"
	"code.cloudfoundry.org/bbs/migration"
	"code.cloudfoundry.org/bbs/models"
	"code.cloudfoundry.org/lager/lagertest"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Split Desired LRP Migration", func() {
	var (
		migration  migration.Migration
		serializer format.Serializer
		cryptor    encryption.Cryptor

		logger *lagertest.TestLogger
	)

	BeforeEach(func() {
		logger = lagertest.NewTestLogger("test")

		encryptionKey, err := encryption.NewKey("label", "passphrase")
		Expect(err).NotTo(HaveOccurred())
		keyManager, err := encryption.NewKeyManager(encryptionKey, nil)
		Expect(err).NotTo(HaveOccurred())
		cryptor = encryption.NewCryptor(keyManager, rand.Reader)
		serializer = format.NewSerializer(cryptor)
		migration = migrations.NewSplitDesiredLRP()
	})

	It("appends itself to the migration list", func() {
		Expect(migrations.Migrations).To(ContainElement(migration))
	})

	Describe("Version", func() {
		It("returns the timestamp from which it was created", func() {
			Expect(migration.Version()).To(BeEquivalentTo(1442529338))
		})
	})

	var newValidDesiredLRP = func(guid string) *models.DesiredLRP {
		myRouterJSON := json.RawMessage(`{"foo":"bar"}`)
		modTag := models.NewModificationTag("epoch", 0)
		desiredLRP := &models.DesiredLRP{
			ProcessGuid:             guid,
			Domain:                  "some-domain",
			RootFs:                  "some:rootfs",
			Instances:               1,
			EnvironmentVariables:    []*models.EnvironmentVariable{{Name: "FOO", Value: "bar"}},
			Setup:                   models.WrapAction(&models.RunAction{Path: "ls", User: "name"}),
			Action:                  models.WrapAction(&models.RunAction{Path: "ls", User: "name"}),
			DeprecatedStartTimeoutS: 15,
			Monitor: models.WrapAction(models.EmitProgressFor(
				models.Timeout(models.Try(models.Parallel(models.Serial(&models.RunAction{Path: "ls", User: "name"}))),
					10*time.Second,
				),
				"start-message",
				"success-message",
				"failure-message",
			)),
			DiskMb:      512,
			MemoryMb:    1024,
			CpuWeight:   42,
			Routes:      &models.Routes{"my-router": &myRouterJSON},
			LogSource:   "some-log-source",
			LogGuid:     "some-log-guid",
			MetricsGuid: "some-metrics-guid",
			Annotation:  "some-annotation",
			EgressRules: []*models.SecurityGroupRule{{
				Protocol:     models.TCPProtocol,
				Destinations: []string{"1.1.1.1/32", "2.2.2.2/32"},
				PortRange:    &models.PortRange{Start: 10, End: 16000},
			}},
			ModificationTag: &modTag,
		}
		err := desiredLRP.Validate()
		Expect(err).NotTo(HaveOccurred())

		return desiredLRP
	}

	Describe("Up", func() {
		var (
			existingDesiredLRP *models.DesiredLRP
			migrationErr       error
		)

		BeforeEach(func() {
			existingDesiredLRP = newValidDesiredLRP("process-guid")
			payload, err := serializer.Marshal(logger, format.ENCRYPTED_PROTO, existingDesiredLRP)
			Expect(err).NotTo(HaveOccurred())
			_, err = storeClient.Set(deprecations.DesiredLRPSchemaPath(existingDesiredLRP), payload, 0)
			Expect(err).NotTo(HaveOccurred())
		})

		JustBeforeEach(func() {
			migration.SetStoreClient(storeClient)
			migration.SetCryptor(cryptor)
			migrationErr = migration.Up(logger)
		})

		It("creates a DesiredLRPSchedulingInfo for all desired LRPs", func() {
			Expect(migrationErr).NotTo(HaveOccurred())

			response, err := storeClient.Get(etcd.DesiredLRPSchedulingInfoSchemaRoot, false, true)
			Expect(err).NotTo(HaveOccurred())
			Expect(response.Node.Nodes).To(HaveLen(1))

			for _, node := range response.Node.Nodes {
				var schedulingInfo models.DesiredLRPSchedulingInfo
				serializer.Unmarshal(logger, []byte(node.Value), &schedulingInfo)

				Expect(schedulingInfo.DesiredLRPKey).To(Equal(existingDesiredLRP.DesiredLRPKey()))
				Expect(schedulingInfo.DesiredLRPResource).To(Equal(existingDesiredLRP.DesiredLRPResource()))
				Expect(schedulingInfo.Annotation).To(Equal(existingDesiredLRP.Annotation))
				Expect(schedulingInfo.Instances).To(Equal(existingDesiredLRP.Instances))
				Expect(schedulingInfo.Routes).To(Equal(*existingDesiredLRP.Routes))
				Expect(schedulingInfo.ModificationTag).To(Equal(*existingDesiredLRP.ModificationTag))
			}
		})

		It("creates a DesiredLRPRunInfo for all desired LRPs", func() {
			Expect(migrationErr).NotTo(HaveOccurred())

			response, err := storeClient.Get(etcd.DesiredLRPRunInfoSchemaRoot, false, true)
			Expect(err).NotTo(HaveOccurred())
			Expect(response.Node.Nodes).To(HaveLen(1))

			for _, node := range response.Node.Nodes {
				var runInfo models.DesiredLRPRunInfo
				serializer.Unmarshal(logger, []byte(node.Value), &runInfo)

				existingEnvVars := make([]models.EnvironmentVariable, len(existingDesiredLRP.EnvironmentVariables))
				for i := range existingDesiredLRP.EnvironmentVariables {
					existingEnvVars[i] = *existingDesiredLRP.EnvironmentVariables[i]
				}

				existingEgressRules := make([]models.SecurityGroupRule, len(existingDesiredLRP.EgressRules))
				for i := range existingDesiredLRP.EgressRules {
					existingEgressRules[i] = *existingDesiredLRP.EgressRules[i]
				}

				Expect(runInfo.DesiredLRPKey).To(Equal(existingDesiredLRP.DesiredLRPKey()))
				Expect(runInfo.EnvironmentVariables).To(Equal(existingEnvVars))
				Expect(runInfo.Setup).To(Equal(existingDesiredLRP.Setup))
				Expect(runInfo.Action).To(Equal(existingDesiredLRP.Action))
				Expect(runInfo.Monitor).To(Equal(existingDesiredLRP.Monitor))
				Expect(runInfo.StartTimeoutMs).To(Equal(existingDesiredLRP.StartTimeoutMs))
				Expect(runInfo.Privileged).To(Equal(existingDesiredLRP.Privileged))
				Expect(runInfo.CpuWeight).To(Equal(existingDesiredLRP.CpuWeight))
				Expect(runInfo.Ports).To(Equal(existingDesiredLRP.Ports))
				Expect(runInfo.EgressRules).To(Equal(existingEgressRules))
				Expect(runInfo.LogSource).To(Equal(existingDesiredLRP.LogSource))
				Expect(runInfo.MetricsGuid).To(Equal(existingDesiredLRP.MetricsGuid))
			}
		})

		It("deletes the desired LRPs afterwards", func() {
			Expect(migrationErr).NotTo(HaveOccurred())

			_, err := storeClient.Get(deprecations.DesiredLRPSchemaPath(existingDesiredLRP), false, true)
			Expect(err).To(HaveOccurred())
		})

		Context("when there are no old desired lrps in the database", func() {
			Context("because the root node does not exist", func() {
				BeforeEach(func() {
					_, err := storeClient.Delete(deprecations.DesiredLRPSchemaRoot, true)
					Expect(err).NotTo(HaveOccurred())
				})

				It("continues the migration", func() {
					Expect(migrationErr).NotTo(HaveOccurred())
				})
			})
		})

		Context("when there are already 'migrated' data in the database", func() {
			var existingSplit *models.DesiredLRP
			BeforeEach(func() {
				existingSplit = newValidDesiredLRP("existing-split")
				schedulingInfo, runInfo := existingSplit.CreateComponents(time.Unix(42, 42))

				schedulingInfoPayload, err := serializer.Marshal(logger, format.ENCRYPTED_PROTO, &schedulingInfo)
				Expect(err).NotTo(HaveOccurred())
				runInfoPayload, err := serializer.Marshal(logger, format.ENCRYPTED_PROTO, &runInfo)
				Expect(err).NotTo(HaveOccurred())

				_, err = storeClient.Set(etcd.DesiredLRPSchedulingInfoSchemaPath("existing-split"), schedulingInfoPayload, 0)
				Expect(err).NotTo(HaveOccurred())
				_, err = storeClient.Set(etcd.DesiredLRPRunInfoSchemaPath("existing-split"), runInfoPayload, 0)
				Expect(err).NotTo(HaveOccurred())
			})

			It("deletes the existing lrp that was already split afterwards", func() {
				Expect(migrationErr).NotTo(HaveOccurred())

				_, err := storeClient.Get(etcd.DesiredLRPSchedulingInfoSchemaPath("existing-split"), false, true)
				Expect(err).To(HaveOccurred())

				_, err = storeClient.Get(etcd.DesiredLRPRunInfoSchemaPath("existing-split"), false, true)
				Expect(err).To(HaveOccurred())
			})
		})
	})

	Describe("Down", func() {
		It("returns a not implemented error", func() {
			Expect(migration.Down(logger)).To(HaveOccurred())
		})
	})
})

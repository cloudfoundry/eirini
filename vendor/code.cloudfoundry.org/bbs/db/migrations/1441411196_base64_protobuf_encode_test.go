package migrations_test

import (
	"crypto/rand"
	"encoding/base64"
	"encoding/json"

	"code.cloudfoundry.org/bbs/db/deprecations"
	"code.cloudfoundry.org/bbs/db/etcd"
	"code.cloudfoundry.org/bbs/db/migrations"
	"code.cloudfoundry.org/bbs/encryption"
	"code.cloudfoundry.org/bbs/format"
	"code.cloudfoundry.org/bbs/migration"
	"code.cloudfoundry.org/bbs/models"
	"code.cloudfoundry.org/bbs/models/test/model_helpers"
	"code.cloudfoundry.org/lager/lagertest"
	goetcd "github.com/coreos/go-etcd/etcd"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Base 64 Protobuf Encode Migration", func() {
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
		migration = migrations.NewBase64ProtobufEncode()
	})

	It("appends itself to the migration list", func() {
		Expect(migrations.Migrations).To(ContainElement(migration))
	})

	Describe("Version", func() {
		It("returns the timestamp from which it was created", func() {
			Expect(migration.Version()).To(BeEquivalentTo(1441411196))
		})
	})

	Describe("Up", func() {
		var (
			expectedDesiredLRP                             *models.DesiredLRP
			expectedActualLRP, expectedEvacuatingActualLRP *models.ActualLRP
			expectedTask                                   *models.Task
			migrationErr                                   error
		)

		BeforeEach(func() {
			// DesiredLRP
			expectedDesiredLRP = model_helpers.NewValidDesiredLRP("process-guid")
			jsonValue, err := json.Marshal(expectedDesiredLRP)
			Expect(err).NotTo(HaveOccurred())
			_, err = storeClient.Set(deprecations.DesiredLRPSchemaPath(expectedDesiredLRP), jsonValue, 0)
			Expect(err).NotTo(HaveOccurred())

			// ActualLRP
			expectedActualLRP = model_helpers.NewValidActualLRP("process-guid", 1)
			jsonValue, err = json.Marshal(expectedActualLRP)
			Expect(err).NotTo(HaveOccurred())
			_, err = storeClient.Set(etcd.ActualLRPSchemaPath(expectedActualLRP.ProcessGuid, 1), jsonValue, 0)
			Expect(err).NotTo(HaveOccurred())

			// Evacuating ActualLRP
			expectedEvacuatingActualLRP = model_helpers.NewValidActualLRP("process-guid", 4)
			jsonValue, err = json.Marshal(expectedEvacuatingActualLRP)
			Expect(err).NotTo(HaveOccurred())
			_, err = storeClient.Set(
				etcd.EvacuatingActualLRPSchemaPath(expectedEvacuatingActualLRP.ProcessGuid, 1),
				jsonValue,
				0,
			)
			Expect(err).NotTo(HaveOccurred())

			// Tasks
			expectedTask = model_helpers.NewValidTask("task-guid")
			jsonValue, err = json.Marshal(expectedTask)
			Expect(err).NotTo(HaveOccurred())
			_, err = storeClient.Set(etcd.TaskSchemaPath(expectedTask), jsonValue, 0)
			Expect(err).NotTo(HaveOccurred())
		})

		JustBeforeEach(func() {
			migration.SetStoreClient(storeClient)
			migration.SetCryptor(cryptor)
			migrationErr = migration.Up(logger)
		})

		var validateConversionToProto = func(node *goetcd.Node, actual, expected format.Versioner) {
			value := node.Value

			Expect(value[:2]).To(BeEquivalentTo(format.BASE64[:]))
			payload, err := base64.StdEncoding.DecodeString(string(value[2:]))
			Expect(err).NotTo(HaveOccurred())
			Expect(payload[0]).To(BeEquivalentTo(format.PROTO))
			serializer.Unmarshal(logger, []byte(value), actual)
			Expect(actual).To(Equal(expected))
		}

		It("converts all data stored in the etcd store to base 64 protobuf", func() {
			Expect(migrationErr).NotTo(HaveOccurred())

			By("Converting DesiredLRPs to Encoded Proto")
			response, err := storeClient.Get(deprecations.DesiredLRPSchemaRoot, false, true)
			Expect(err).NotTo(HaveOccurred())
			Expect(response.Node.Nodes).To(HaveLen(1))
			for _, node := range response.Node.Nodes {
				var desiredLRP models.DesiredLRP
				value := node.Value
				err := serializer.Unmarshal(logger, []byte(value), &desiredLRP)
				Expect(err).NotTo(HaveOccurred())
				validateConversionToProto(node, &desiredLRP, expectedDesiredLRP)
			}

			By("Converting ActualLRPs to Encoded Proto")
			response, err = storeClient.Get(etcd.ActualLRPSchemaRoot, false, true)
			Expect(err).NotTo(HaveOccurred())
			Expect(response.Node.Nodes).To(HaveLen(1))
			for _, processNode := range response.Node.Nodes {
				for _, groupNode := range processNode.Nodes {
					for _, lrpNode := range groupNode.Nodes {
						var expected *models.ActualLRP
						if lrpNode.Key == etcd.ActualLRPSchemaPath("process-guid", 1) {
							expected = expectedActualLRP
						} else {
							expected = expectedEvacuatingActualLRP
						}
						var actualLRP models.ActualLRP
						serializer.Unmarshal(logger, []byte(lrpNode.Value), &actualLRP)
						validateConversionToProto(lrpNode, &actualLRP, expected)
					}
				}
			}

			By("Converting Tasks to Encoded Proto")
			response, err = storeClient.Get(etcd.TaskSchemaRoot, false, true)
			Expect(err).NotTo(HaveOccurred())
			Expect(response.Node.Nodes).To(HaveLen(1))
			for _, taskNode := range response.Node.Nodes {
				var task models.Task
				serializer.Unmarshal(logger, []byte(taskNode.Value), &task)
				validateConversionToProto(taskNode, &task, expectedTask)
			}
		})

		Context("when fetching desired lrps fails", func() {
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

		Context("when fetching actual lrps fails", func() {
			Context("because the root node does not exist", func() {
				BeforeEach(func() {
					_, err := storeClient.Delete(etcd.ActualLRPSchemaRoot, true)
					Expect(err).NotTo(HaveOccurred())
				})

				It("continues the migration", func() {
					Expect(migrationErr).NotTo(HaveOccurred())
				})
			})
		})

		Context("when fetching tasks fails", func() {
			Context("because the root node does not exist", func() {
				BeforeEach(func() {
					_, err := storeClient.Delete(etcd.TaskSchemaRoot, true)
					Expect(err).NotTo(HaveOccurred())
				})

				It("continues the migration", func() {
					Expect(migrationErr).NotTo(HaveOccurred())
				})
			})
		})
	})

	Describe("Down", func() {
		It("returns a not implemented error", func() {
			Expect(migration.Down(logger)).To(HaveOccurred())
		})
	})
})

package bifrost_test

import (
	"context"
	"encoding/json"
	"errors"

	"code.cloudfoundry.org/bbs/models"
	"code.cloudfoundry.org/eirini"
	"code.cloudfoundry.org/eirini/bifrost"
	"code.cloudfoundry.org/eirini/bifrost/bifrostfakes"
	"code.cloudfoundry.org/eirini/models/cf"
	"code.cloudfoundry.org/eirini/opi"
	"code.cloudfoundry.org/eirini/opi/opifakes"
	"code.cloudfoundry.org/lager"
	"code.cloudfoundry.org/lager/lagertest"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Bifrost", func() {

	var (
		err       error
		bfrst     eirini.Bifrost
		request   cf.DesireLRPRequest
		converter *bifrostfakes.FakeConverter
		desirer   *opifakes.FakeDesirer
		lager     lager.Logger
		opiClient *opifakes.FakeDesirer
	)

	Context("Transfer", func() {

		BeforeEach(func() {
			converter = new(bifrostfakes.FakeConverter)
			desirer = new(opifakes.FakeDesirer)
		})

		JustBeforeEach(func() {
			bfrst = &bifrost.Bifrost{
				Converter: converter,
				Desirer:   desirer,
				Logger:    lagertest.NewTestLogger("bifrost"),
			}
			err = bfrst.Transfer(context.Background(), request)
		})

		Context("When lrp is transferred succesfully", func() {
			var lrp opi.LRP

			BeforeEach(func() {
				lrp = opi.LRP{
					Image: "docker.png",
				}
				converter.ConvertReturns(lrp, nil)
			})

			It("should not return an error", func() {
				Expect(err).ToNot(HaveOccurred())
			})

			It("should use Converter", func() {
				Expect(converter.ConvertCallCount()).To(Equal(1))
				Expect(converter.ConvertArgsForCall(0)).To(Equal(request))
			})

			It("should use Desirer with the converted LRP", func() {
				Expect(desirer.DesireCallCount()).To(Equal(1))
				desired := desirer.DesireArgsForCall(0)
				Expect(desired).To(Equal(&lrp))
			})
		})

		Context("When lrp transfer fails", func() {
			Context("when Converter fails", func() {
				BeforeEach(func() {
					converter.ConvertReturns(opi.LRP{}, errors.New("failed-to-convert"))
				})

				It("shoud return an error", func() {
					Expect(err).To(HaveOccurred())
				})

				It("should use Converter", func() {
					Expect(converter.ConvertCallCount()).To(Equal(1))
					Expect(converter.ConvertArgsForCall(0)).To(Equal(request))
				})

				It("should not use Desirer", func() {
					Expect(desirer.DesireCallCount()).To(Equal(0))
				})
			})

			Context("When Desirer fails", func() {
				BeforeEach(func() {
					desirer.DesireReturns(errors.New("failed-to-desire"))
				})

				It("shoud return an error", func() {
					Expect(err).To(HaveOccurred())
				})
			})
		})

	})

	Context("List", func() {
		var (
			lrps                      []*opi.LRP
			desiredLRPSchedulingInfos []*models.DesiredLRPSchedulingInfo
		)

		createLRP := func(processGUID, lastUpdated string) *opi.LRP {
			return &opi.LRP{
				Metadata: map[string]string{
					cf.ProcessGUID: processGUID,
					cf.LastUpdated: lastUpdated,
				},
			}
		}

		BeforeEach(func() {
			opiClient = new(opifakes.FakeDesirer)
			lager = lagertest.NewTestLogger("bifrost-test")
			bfrst = &bifrost.Bifrost{
				Desirer: opiClient,
				Logger:  lager,
			}
			lrps = []*opi.LRP{}
		})

		JustBeforeEach(func() {
			desiredLRPSchedulingInfos, err = bfrst.List(context.Background())
		})

		Context("When no running LRPs exist", func() {

			It("should not return an error", func() {
				Expect(err).ToNot(HaveOccurred())
			})

			It("should return an empty list of DesiredLRPSchedulingInfo", func() {
				Expect(len(desiredLRPSchedulingInfos)).To(Equal(0))
			})
		})

		Context("When listing running LRPs", func() {

			BeforeEach(func() {
				lrps = []*opi.LRP{
					createLRP("abcd", "3464634.2"),
					createLRP("efgh", "235.26535"),
					createLRP("ijkl", "2342342.2"),
				}
				opiClient.ListReturns(lrps, nil)
			})

			It("should not return an error", func() {
				Expect(err).ToNot(HaveOccurred())
			})

			It("should translate []LRPs to []DesiredLRPSchedulingInfo", func() {
				Expect(desiredLRPSchedulingInfos[0].ProcessGuid).To(Equal("abcd"))
				Expect(desiredLRPSchedulingInfos[1].ProcessGuid).To(Equal("efgh"))
				Expect(desiredLRPSchedulingInfos[2].ProcessGuid).To(Equal("ijkl"))

				Expect(desiredLRPSchedulingInfos[0].Annotation).To(Equal("3464634.2"))
				Expect(desiredLRPSchedulingInfos[1].Annotation).To(Equal("235.26535"))
				Expect(desiredLRPSchedulingInfos[2].Annotation).To(Equal("2342342.2"))
			})
		})

		Context("When an error occurs", func() {

			BeforeEach(func() {
				opiClient.ListReturns(nil, errors.New("arrgh"))
			})

			It("should return a meaningful errormessage", func() {
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).Should(ContainSubstring("failed to list desired LRPs"))
			})
		})
	})

	Context("Update an app", func() {

		var (
			bfrst         bifrost.Bifrost
			updateRequest cf.UpdateDesiredLRPRequest
		)

		BeforeEach(func() {
			updateRequest = cf.UpdateDesiredLRPRequest{
				GUID:    "guid_1234",
				Version: "version_1234",
			}
			opiClient = new(opifakes.FakeDesirer)

			lager = lagertest.NewTestLogger("bifrost-update-test")
		})

		JustBeforeEach(func() {
			bfrst = bifrost.Bifrost{
				Desirer: opiClient,
				Logger:  lager,
			}

			err = bfrst.Update(context.Background(), updateRequest)
		})

		Context("when the app exists", func() {

			BeforeEach(func() {
				lrp := opi.LRP{
					TargetInstances: 2,
					Metadata: map[string]string{
						cf.LastUpdated: "whenever",
						cf.VcapAppUris: `[{"hostname":"my.route","port":8080},{"hostname":"your.route","port":5555}]`,
					},
				}
				opiClient.GetReturns(&lrp, nil)
			})

			Context("with instance count modified", func() {

				BeforeEach(func() {
					updatedInstances := int32(5)
					updatedTimestamp := "21421321.3"
					updateRequest.Update = &models.DesiredLRPUpdate{Instances: &updatedInstances, Annotation: &updatedTimestamp}
					opiClient.UpdateReturns(nil)
				})

				It("should get the existing LRP", func() {
					Expect(opiClient.GetCallCount()).To(Equal(1))
					identifier := opiClient.GetArgsForCall(0)
					Expect(identifier.GUID).To(Equal("guid_1234"))
					Expect(identifier.Version).To(Equal("version_1234"))
				})

				It("should submit the updated LRP", func() {
					Expect(opiClient.UpdateCallCount()).To(Equal(1))
					lrp := opiClient.UpdateArgsForCall(0)
					Expect(lrp.TargetInstances).To(Equal(int(*updateRequest.Update.Instances)))
					Expect(lrp.Metadata[cf.LastUpdated]).To(Equal("21421321.3"))
				})

				It("should not return an error", func() {
					Expect(err).ToNot(HaveOccurred())
				})

				Context("when the update fails", func() {
					BeforeEach(func() {
						opiClient.UpdateReturns(errors.New("failed to update app"))
					})

					It("should propagate the error", func() {
						Expect(err).To(HaveOccurred())
					})
				})
			})

			Context("When the routes are updated", func() {
				BeforeEach(func() {
					updatedRoutes := []map[string]interface{}{
						{
							"hostname": "my.route",
							"port":     8080,
						},
						{
							"hostname": "my.other.route",
							"port":     7777,
						},
					}

					routesJSON, marshalErr := json.Marshal(updatedRoutes)
					Expect(marshalErr).ToNot(HaveOccurred())

					rawJSON := json.RawMessage(routesJSON)

					updatedInstances := int32(5)
					updatedTimestamp := "23456.7"
					updateRequest.Update = &models.DesiredLRPUpdate{
						Routes: &models.Routes{
							"cf-router": &rawJSON,
						},
						Instances:  &updatedInstances,
						Annotation: &updatedTimestamp,
					}

					opiClient.UpdateReturns(nil)
				})

				It("should get the existing LRP", func() {
					Expect(opiClient.GetCallCount()).To(Equal(1))
					identifier := opiClient.GetArgsForCall(0)
					Expect(identifier.GUID).To(Equal("guid_1234"))
					Expect(identifier.Version).To(Equal("version_1234"))
				})

				It("should have the updated routes", func() {
					Expect(opiClient.UpdateCallCount()).To(Equal(1))
					lrp := opiClient.UpdateArgsForCall(0)
					Expect(lrp.Metadata[cf.VcapAppUris]).To(Equal(`[{"hostname":"my.route","port":8080},{"hostname":"my.other.route","port":7777}]`))
				})

				Context("When there are no routes provided", func() {
					BeforeEach(func() {
						updatedRoutes := []map[string]interface{}{}

						routesJSON, marshalErr := json.Marshal(updatedRoutes)
						Expect(marshalErr).ToNot(HaveOccurred())

						rawJSON := json.RawMessage(routesJSON)
						updateRequest.Update.Routes = &models.Routes{
							"cf-router": &rawJSON,
						}
					})

					It("should update it to an empty array", func() {
						Expect(opiClient.UpdateCallCount()).To(Equal(1))
						lrp := opiClient.UpdateArgsForCall(0)
						Expect(lrp.Metadata[cf.VcapAppUris]).To(Equal(`[]`))
					})
				})
			})
		})

		Context("when the app does not exist", func() {

			BeforeEach(func() {
				opiClient.GetReturns(nil, errors.New("app does not exist"))
			})

			It("should try to get the LRP", func() {
				Expect(opiClient.GetCallCount()).To(Equal(1))
				identifier := opiClient.GetArgsForCall(0)
				Expect(identifier.GUID).To(Equal("guid_1234"))
				Expect(identifier.Version).To(Equal("version_1234"))

			})

			It("should not submit anything to be updated", func() {
				Expect(opiClient.UpdateCallCount()).To(Equal(0))
			})

			It("should propagate the error", func() {
				Expect(err).To(HaveOccurred())
			})
		})
	})

	Context("get an App", func() {
		var (
			desiredLRP *models.DesiredLRP
			lrp        *opi.LRP
		)

		BeforeEach(func() {
			opiClient = new(opifakes.FakeDesirer)

			lager = lagertest.NewTestLogger("bifrost-update-test")
		})

		JustBeforeEach(func() {
			bfrst = &bifrost.Bifrost{
				Desirer: opiClient,
				Logger:  lager,
			}
			identifier := opi.LRPIdentifier{
				GUID:    "guid_1234",
				Version: "version_1234",
			}

			desiredLRP = bfrst.GetApp(context.Background(), identifier)
		})

		Context("when the app exists", func() {
			BeforeEach(func() {
				lrp = &opi.LRP{
					TargetInstances: 5,
				}

				opiClient.GetReturns(lrp, nil)
			})

			It("should use the desirer to get the lrp", func() {
				Expect(opiClient.GetCallCount()).To(Equal(1))
				identifier := opiClient.GetArgsForCall(0)
				Expect(identifier.GUID).To(Equal("guid_1234"))
				Expect(identifier.Version).To(Equal("version_1234"))
			})

			It("should return a DesiredLRP", func() {
				Expect(desiredLRP).ToNot(BeNil())
				Expect(desiredLRP.ProcessGuid).To(Equal("guid_1234-version_1234"))
				Expect(desiredLRP.Instances).To(Equal(int32(5)))
			})
		})

		Context("when the app does not exist", func() {
			BeforeEach(func() {
				opiClient.GetReturns(nil, errors.New("Failed to get LRP"))
			})

			It("should return an error", func() {
				Expect(opiClient.GetCallCount()).To(Equal(1))
				Expect(desiredLRP).To(BeNil())
			})
		})
	})

	Context("Stop an app", func() {
		BeforeEach(func() {
			opiClient = new(opifakes.FakeDesirer)

			lager = lagertest.NewTestLogger("bifrost-stop-test")
			bfrst = &bifrost.Bifrost{
				Desirer: opiClient,
				Logger:  lager,
			}
		})

		JustBeforeEach(func() {
			err = bfrst.Stop(context.Background(), opi.LRPIdentifier{GUID: "guid_1234", Version: "version_1234"})
		})

		It("should not return an error", func() {
			Expect(err).ToNot(HaveOccurred())
		})

		It("should call the desirer with the expected guid", func() {
			identifier := opiClient.StopArgsForCall(0)
			Expect(identifier.GUID).To(Equal("guid_1234"))
			Expect(identifier.Version).To(Equal("version_1234"))
		})

		Context("when desirer's stop fails", func() {

			BeforeEach(func() {
				opiClient.StopReturns(errors.New("failed-to-stop"))
			})

			It("returns an error", func() {
				Expect(err).To(HaveOccurred())
			})
		})
	})

	Context("Get all instances of an app", func() {
		var (
			instances    []*cf.Instance
			opiInstances []*opi.Instance
		)

		BeforeEach(func() {
			opiClient = new(opifakes.FakeDesirer)
			lager = lagertest.NewTestLogger("bifrost-get-instances-test")
			opiInstances = []*opi.Instance{
				{Index: 0, Since: 123, State: opi.RunningState},
				{Index: 1, Since: 345, State: opi.CrashedState},
				{Index: 2, Since: 678, State: opi.ErrorState, PlacementError: "this is not the place"},
			}

			opiClient.GetInstancesReturns(opiInstances, nil)
		})

		JustBeforeEach(func() {
			bfrst = &bifrost.Bifrost{
				Desirer: opiClient,
				Logger:  lager,
			}

			instances, err = bfrst.GetInstances(context.Background(), opi.LRPIdentifier{GUID: "guid_1234", Version: "version_1234"})
		})

		It("should get the app instances from Desirer", func() {
			Expect(opiClient.GetInstancesCallCount()).To(Equal(1))
			identifier := opiClient.GetInstancesArgsForCall(0)
			Expect(identifier.GUID).To(Equal("guid_1234"))
			Expect(identifier.Version).To(Equal("version_1234"))
		})

		It("should not return an error", func() {
			Expect(err).ToNot(HaveOccurred())
		})

		It("should return all running instances", func() {
			Expect(instances).To(Equal([]*cf.Instance{
				{Index: 0, Since: 123, State: opi.RunningState},
				{Index: 1, Since: 345, State: opi.CrashedState},
				{Index: 2, Since: 678, State: opi.ErrorState, PlacementError: "this is not the place"},
			}))
		})

		Context("when the app does not exist", func() {
			BeforeEach(func() {
				opiClient.GetInstancesReturns([]*opi.Instance{}, errors.New("not found"))
			})

			It("returns an error", func() {
				Expect(err).To(HaveOccurred())
			})
		})

	})
})

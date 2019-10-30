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
	)

	BeforeEach(func() {
		converter = new(bifrostfakes.FakeConverter)
		desirer = new(opifakes.FakeDesirer)
	})

	JustBeforeEach(func() {
		bfrst = &bifrost.Bifrost{
			Converter: converter,
			Desirer:   desirer,
		}
	})

	Context("Transfer", func() {

		Context("When lrp is transferred successfully", func() {
			var lrp opi.LRP
			JustBeforeEach(func() {
				err = bfrst.Transfer(context.Background(), request)
			})

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
					request = cf.DesireLRPRequest{GUID: "my-guid"}
					converter.ConvertReturns(opi.LRP{}, errors.New("failed-to-convert"))
				})

				It("should return an error", func() {
					Expect(bfrst.Transfer(context.Background(), request)).To(MatchError(ContainSubstring("failed to convert request")))
				})

				It("should use Converter", func() {
					Expect(bfrst.Transfer(context.Background(), request)).ToNot(Succeed())
					Expect(converter.ConvertCallCount()).To(Equal(1))
					Expect(converter.ConvertArgsForCall(0)).To(Equal(request))
				})

				It("should not use Desirer", func() {
					Expect(bfrst.Transfer(context.Background(), request)).ToNot(Succeed())
					Expect(desirer.DesireCallCount()).To(Equal(0))
				})

			})

			Context("When Desirer fails", func() {
				BeforeEach(func() {
					desirer.DesireReturns(errors.New("failed-to-desire-main-error"))
				})

				It("shoud return an error", func() {
					Expect(bfrst.Transfer(context.Background(), request)).To(MatchError(ContainSubstring("failed to desire")))
				})
			})
		})

	})

	Context("List", func() {
		createLRP := func(processGUID, lastUpdated string) *opi.LRP {
			return &opi.LRP{
				LRPIdentifier: opi.LRPIdentifier{GUID: processGUID},
				LastUpdated:   lastUpdated,
			}
		}

		Context("When no running LRPs exist", func() {
			It("should return an empty list of DesiredLRPSchedulingInfo", func() {
				Expect(bfrst.List(context.Background())).To(HaveLen(0))
			})
		})

		Context("When listing running LRPs", func() {
			BeforeEach(func() {
				lrps := []*opi.LRP{
					createLRP("abcd", "3464634.2"),
					createLRP("efgh", "235.26535"),
					createLRP("ijkl", "2342342.2"),
				}
				desirer.ListReturns(lrps, nil)
			})

			It("should succeed", func() {
				_, listErr := bfrst.List(context.Background())
				Expect(listErr).ToNot(HaveOccurred())
			})

			It("should translate []LRPs to []DesiredLRPSchedulingInfo", func() {
				desiredLRPSchedulingInfos, _ := bfrst.List(context.Background())
				Expect(desiredLRPSchedulingInfos).To(HaveLen(3))
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
				desirer.ListReturns(nil, errors.New("arrgh"))
			})

			It("should return a meaningful errormessage", func() {
				_, listErr := bfrst.List(context.Background())
				Expect(listErr).To(MatchError(ContainSubstring("failed to list desired LRPs")))
			})
		})
	})

	Context("Update an app", func() {

		var (
			updateRequest cf.UpdateDesiredLRPRequest
		)

		BeforeEach(func() {
			updateRequest = cf.UpdateDesiredLRPRequest{
				GUID:    "guid_1234",
				Version: "version_1234",
			}
		})

		JustBeforeEach(func() {
			err = bfrst.Update(context.Background(), updateRequest)
		})

		Context("when the app exists", func() {

			BeforeEach(func() {
				lrp := opi.LRP{
					TargetInstances: 2,
					LastUpdated:     "whenever",
					AppURIs:         `[{"hostname":"my.route","port":8080},{"hostname":"your.route","port":5555}]`,
				}
				desirer.GetReturns(&lrp, nil)
			})

			Context("with instance count modified", func() {
				BeforeEach(func() {
					updateRequest.Update = &models.DesiredLRPUpdate{}
					updateRequest.Update.SetInstances(int32(5))
					updateRequest.Update.SetAnnotation("21421321.3")
					desirer.UpdateReturns(nil)
				})

				It("should get the existing LRP", func() {
					Expect(desirer.GetCallCount()).To(Equal(1))
					identifier := desirer.GetArgsForCall(0)
					Expect(identifier.GUID).To(Equal("guid_1234"))
					Expect(identifier.Version).To(Equal("version_1234"))
				})

				It("should submit the updated LRP", func() {
					Expect(desirer.UpdateCallCount()).To(Equal(1))
					lrp := desirer.UpdateArgsForCall(0)
					Expect(lrp.TargetInstances).To(Equal(int(5)))
					Expect(lrp.LastUpdated).To(Equal("21421321.3"))
				})

				It("should not return an error", func() {
					Expect(err).ToNot(HaveOccurred())
				})

				Context("when the update fails", func() {
					BeforeEach(func() {
						desirer.UpdateReturns(errors.New("your app is bad"))
					})

					It("should propagate the error", func() {
						Expect(err).To(MatchError(ContainSubstring("failed to update")))
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
					}
					updateRequest.Update.SetInstances(updatedInstances)
					updateRequest.Update.SetAnnotation(updatedTimestamp)

					desirer.UpdateReturns(nil)
				})

				It("should get the existing LRP", func() {
					Expect(desirer.GetCallCount()).To(Equal(1))
					identifier := desirer.GetArgsForCall(0)
					Expect(identifier.GUID).To(Equal("guid_1234"))
					Expect(identifier.Version).To(Equal("version_1234"))
				})

				It("should have the updated routes", func() {
					Expect(desirer.UpdateCallCount()).To(Equal(1))
					lrp := desirer.UpdateArgsForCall(0)
					Expect(lrp.AppURIs).To(Equal(`[{"hostname":"my.route","port":8080},{"hostname":"my.other.route","port":7777}]`))
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
						Expect(desirer.UpdateCallCount()).To(Equal(1))
						lrp := desirer.UpdateArgsForCall(0)
						Expect(lrp.AppURIs).To(Equal(`[]`))
					})
				})
			})
		})

		Context("when the app does not exist", func() {

			BeforeEach(func() {
				desirer.GetReturns(nil, errors.New("app does not exist"))
			})

			It("should try to get the LRP", func() {
				Expect(desirer.GetCallCount()).To(Equal(1))
				identifier := desirer.GetArgsForCall(0)
				Expect(identifier.GUID).To(Equal("guid_1234"))
				Expect(identifier.Version).To(Equal("version_1234"))

			})

			It("should not submit anything to be updated", func() {
				Expect(desirer.UpdateCallCount()).To(Equal(0))
			})

			It("should propagate the error", func() {
				Expect(err).To(MatchError(ContainSubstring("failed to get app")))
			})
		})
	})

	Context("Get an App", func() {
		var (
			lrp        *opi.LRP
			identifier opi.LRPIdentifier
		)

		JustBeforeEach(func() {
			identifier = opi.LRPIdentifier{
				GUID:    "guid_1234",
				Version: "version_1234",
			}

		})

		Context("when the app exists", func() {
			BeforeEach(func() {
				lrp = &opi.LRP{
					TargetInstances: 5,
				}

				desirer.GetReturns(lrp, nil)
			})

			It("should use the desirer to get the lrp", func() {
				_, err = bfrst.GetApp(context.Background(), identifier)
				Expect(err).NotTo(HaveOccurred())
				Expect(desirer.GetCallCount()).To(Equal(1))
				Expect(desirer.GetArgsForCall(0)).To(Equal(identifier))
			})

			It("should return a DesiredLRP", func() {
				desiredLRP, _ := bfrst.GetApp(context.Background(), identifier)
				Expect(desiredLRP).ToNot(BeNil())
				Expect(desiredLRP.ProcessGuid).To(Equal("guid_1234-version_1234"))
				Expect(desiredLRP.Instances).To(Equal(int32(5)))
			})

		})

		Context("when the app does not exist", func() {
			BeforeEach(func() {
				desirer.GetReturns(nil, errors.New("Failed to get LRP"))
			})

			It("should return an error", func() {
				_, getAppErr := bfrst.GetApp(context.Background(), identifier)
				Expect(getAppErr).To(MatchError(ContainSubstring("failed to get app")))
			})
		})
	})

	Context("Stop an app", func() {

		JustBeforeEach(func() {
			err = bfrst.Stop(context.Background(), opi.LRPIdentifier{GUID: "guid_1234", Version: "version_1234"})
		})

		It("should not return an error", func() {
			Expect(err).ToNot(HaveOccurred())
		})

		It("should call the desirer with the expected guid", func() {
			identifier := desirer.StopArgsForCall(0)
			Expect(identifier.GUID).To(Equal("guid_1234"))
			Expect(identifier.Version).To(Equal("version_1234"))
		})

		Context("when desirer's stop fails", func() {

			BeforeEach(func() {
				desirer.StopReturns(errors.New("failed-to-stop"))
			})

			It("returns an error", func() {
				Expect(err).To(MatchError(ContainSubstring("failed to stop app")))
			})
		})
	})

	Context("Stop an app instance", func() {

		JustBeforeEach(func() {
			err = bfrst.StopInstance(context.Background(), opi.LRPIdentifier{GUID: "guid_1234", Version: "version_1234"}, 1)
		})

		It("should not return an error", func() {
			Expect(err).ToNot(HaveOccurred())
		})

		It("should call the desirer with the expected guid and index", func() {
			identifier, index := desirer.StopInstanceArgsForCall(0)
			Expect(identifier.GUID).To(Equal("guid_1234"))
			Expect(identifier.Version).To(Equal("version_1234"))
			Expect(index).To(Equal(uint(1)))
		})

		Context("when desirer's stop instance fails", func() {
			BeforeEach(func() {
				desirer.StopInstanceReturns(errors.New("failed-to-stop"))
			})

			It("returns a meaningful error", func() {
				Expect(err).To(MatchError(ContainSubstring("failed to stop instance")))
			})
		})
	})

	Context("Get all instances of an app", func() {
		var (
			instances    []*cf.Instance
			opiInstances []*opi.Instance
		)

		BeforeEach(func() {
			opiInstances = []*opi.Instance{
				{Index: 0, Since: 123, State: opi.RunningState},
				{Index: 1, Since: 345, State: opi.CrashedState},
				{Index: 2, Since: 678, State: opi.ErrorState, PlacementError: "this is not the place"},
			}

			desirer.GetInstancesReturns(opiInstances, nil)
		})

		JustBeforeEach(func() {
			instances, err = bfrst.GetInstances(context.Background(), opi.LRPIdentifier{GUID: "guid_1234", Version: "version_1234"})
		})

		It("should get the app instances from Desirer", func() {
			Expect(desirer.GetInstancesCallCount()).To(Equal(1))
			identifier := desirer.GetInstancesArgsForCall(0)
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
				desirer.GetInstancesReturns([]*opi.Instance{}, errors.New("not found"))
			})

			It("returns an error", func() {
				Expect(err).To(MatchError(ContainSubstring("failed to get instances for app")))
			})
		})

	})
})

package bifrost_test

import (
	"context"
	"encoding/json"
	"errors"

	"code.cloudfoundry.org/eirini/api"
	"code.cloudfoundry.org/eirini/bifrost"
	"code.cloudfoundry.org/eirini/bifrost/bifrostfakes"
	"code.cloudfoundry.org/eirini/models/cf"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Bifrost LRP", func() {
	var (
		err           error
		lrpBifrost    *bifrost.LRP
		request       cf.DesireLRPRequest
		lrpConverter  *bifrostfakes.FakeLRPConverter
		lrpClient     *bifrostfakes.FakeLRPClient
		lrpNamespacer *bifrostfakes.FakeLRPNamespacer
	)

	BeforeEach(func() {
		lrpConverter = new(bifrostfakes.FakeLRPConverter)
		lrpClient = new(bifrostfakes.FakeLRPClient)
		lrpNamespacer = new(bifrostfakes.FakeLRPNamespacer)
		lrpNamespacer.GetNamespaceReturns("my-namespace")

		request = cf.DesireLRPRequest{
			GUID:      "my-guid",
			Namespace: "foo-namespace",
		}
	})

	JustBeforeEach(func() {
		lrpBifrost = &bifrost.LRP{
			Converter:  lrpConverter,
			LRPClient:  lrpClient,
			Namespacer: lrpNamespacer,
		}
	})

	Describe("Transfer LRP", func() {
		Context("When lrp is transferred successfully", func() {
			var lrp api.LRP
			JustBeforeEach(func() {
				err = lrpBifrost.Transfer(context.Background(), request)
			})

			BeforeEach(func() {
				lrp = api.LRP{
					Image: "docker.png",
				}
				lrpConverter.ConvertLRPReturns(lrp, nil)
			})

			It("should not return an error", func() {
				Expect(err).ToNot(HaveOccurred())
			})

			It("should use Converter", func() {
				Expect(lrpConverter.ConvertLRPCallCount()).To(Equal(1))
				Expect(lrpConverter.ConvertLRPArgsForCall(0)).To(Equal(request))
			})

			It("should use LRPClient with the converted LRP", func() {
				Expect(lrpClient.DesireCallCount()).To(Equal(1))
				_, _, desired, _ := lrpClient.DesireArgsForCall(0)
				Expect(desired).To(Equal(&lrp))
			})

			It("should desire the LRP in the requested namespace", func() {
				Expect(lrpClient.DesireCallCount()).To(Equal(1))
				_, namespace, _, _ := lrpClient.DesireArgsForCall(0)
				Expect(namespace).To(Equal("my-namespace"))
			})
		})

		Context("When lrp transfer fails", func() {
			Context("when Converter fails", func() {
				BeforeEach(func() {
					request = cf.DesireLRPRequest{GUID: "my-guid"}
					lrpConverter.ConvertLRPReturns(api.LRP{}, errors.New("failed-to-convert"))
				})

				It("should return an error", func() {
					Expect(lrpBifrost.Transfer(context.Background(), request)).To(MatchError(ContainSubstring("failed to convert request")))
				})

				It("should use Converter", func() {
					Expect(lrpBifrost.Transfer(context.Background(), request)).ToNot(Succeed())
					Expect(lrpConverter.ConvertLRPCallCount()).To(Equal(1))
					Expect(lrpConverter.ConvertLRPArgsForCall(0)).To(Equal(request))
				})

				It("should not use LRPClient", func() {
					Expect(lrpBifrost.Transfer(context.Background(), request)).ToNot(Succeed())
					Expect(lrpClient.DesireCallCount()).To(Equal(0))
				})
			})

			Context("When LRPClient fails", func() {
				BeforeEach(func() {
					lrpClient.DesireReturns(errors.New("failed-to-desire-main-error"))
				})

				It("shoud return an error", func() {
					Expect(lrpBifrost.Transfer(context.Background(), request)).To(MatchError(ContainSubstring("failed to desire")))
				})
			})
		})
	})

	Describe("List LRP", func() {
		createLRP := func(processGUID, version, lastUpdated string) *api.LRP {
			return &api.LRP{
				LRPIdentifier: api.LRPIdentifier{GUID: processGUID, Version: version},
				LastUpdated:   lastUpdated,
			}
		}

		Context("When no running LRPs exist", func() {
			It("should return an empty list of DesiredLRPSchedulingInfo", func() {
				Expect(lrpBifrost.List(context.Background())).To(HaveLen(0))
			})
		})

		Context("When listing running LRPs", func() {
			BeforeEach(func() {
				lrps := []*api.LRP{
					createLRP("abcd", "123", "3464634.2"),
					createLRP("efgh", "234", "235.26535"),
					createLRP("ijkl", "123", "2342342.2"),
				}
				lrpClient.ListReturns(lrps, nil)
			})

			It("should succeed", func() {
				_, listErr := lrpBifrost.List(context.Background())
				Expect(listErr).ToNot(HaveOccurred())
			})

			It("should translate []LRPs to []DesiredLRPSchedulingInfo", func() {
				desiredLRPSchedulingInfos, _ := lrpBifrost.List(context.Background())
				Expect(desiredLRPSchedulingInfos).To(HaveLen(3))
				Expect(desiredLRPSchedulingInfos[0].ProcessGUID).To(Equal("abcd-123"))
				Expect(desiredLRPSchedulingInfos[0].GUID).To(Equal("abcd"))
				Expect(desiredLRPSchedulingInfos[0].Version).To(Equal("123"))
				Expect(desiredLRPSchedulingInfos[0].Annotation).To(Equal("3464634.2"))

				Expect(desiredLRPSchedulingInfos[1].ProcessGUID).To(Equal("efgh-234"))
				Expect(desiredLRPSchedulingInfos[1].GUID).To(Equal("efgh"))
				Expect(desiredLRPSchedulingInfos[1].Version).To(Equal("234"))
				Expect(desiredLRPSchedulingInfos[1].Annotation).To(Equal("235.26535"))

				Expect(desiredLRPSchedulingInfos[2].ProcessGUID).To(Equal("ijkl-123"))
				Expect(desiredLRPSchedulingInfos[2].GUID).To(Equal("ijkl"))
				Expect(desiredLRPSchedulingInfos[2].Version).To(Equal("123"))
				Expect(desiredLRPSchedulingInfos[2].Annotation).To(Equal("2342342.2"))
			})
		})

		Context("When an error occurs", func() {
			BeforeEach(func() {
				lrpClient.ListReturns(nil, errors.New("arrgh"))
			})

			It("should return a meaningful errormessage", func() {
				_, listErr := lrpBifrost.List(context.Background())
				Expect(listErr).To(MatchError(ContainSubstring("failed to list desired LRPs")))
			})
		})
	})

	Describe("Update an app", func() {
		var updateRequest cf.UpdateDesiredLRPRequest

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

			updateRequest = cf.UpdateDesiredLRPRequest{
				GUID:    "guid_1234",
				Version: "version_1234",
				Update: cf.DesiredLRPUpdate{
					Instances:  5,
					Annotation: "21421321.3",
					Routes: map[string]json.RawMessage{
						"cf-router": json.RawMessage(routesJSON),
					},
					Image: "the/image",
				},
			}

			lrpClient.GetReturns(&api.LRP{
				TargetInstances: 2,
				LastUpdated:     "whenever",
				AppURIs: []api.Route{
					{Hostname: "my.route", Port: 8080},
					{Hostname: "your.route", Port: 5555},
				},
			}, nil)

			lrpClient.UpdateReturns(nil)
		})

		JustBeforeEach(func() {
			err = lrpBifrost.Update(context.Background(), updateRequest)
		})

		It("should not return an error", func() {
			Expect(err).ToNot(HaveOccurred())
		})

		It("should get the existing LRP", func() {
			Expect(lrpClient.GetCallCount()).To(Equal(1))
			_, identifier := lrpClient.GetArgsForCall(0)
			Expect(identifier.GUID).To(Equal("guid_1234"))
			Expect(identifier.Version).To(Equal("version_1234"))
		})

		It("should submit the updated LRP", func() {
			Expect(lrpClient.UpdateCallCount()).To(Equal(1))
			_, lrp := lrpClient.UpdateArgsForCall(0)
			Expect(lrp.TargetInstances).To(Equal(int(5)))
			Expect(lrp.LastUpdated).To(Equal("21421321.3"))
			Expect(lrp.AppURIs).To(Equal([]api.Route{
				{Hostname: "my.route", Port: 8080},
				{Hostname: "my.other.route", Port: 7777},
			}))
			Expect(lrp.Image).To(Equal("the/image"))
		})

		Context("when the update fails", func() {
			BeforeEach(func() {
				lrpClient.UpdateReturns(errors.New("your app is bad"))
			})

			It("should propagate the error", func() {
				Expect(err).To(MatchError(ContainSubstring("failed to update")))
			})
		})

		Context("When there are no routes provided", func() {
			BeforeEach(func() {
				updateRequest.Update.Routes = map[string]json.RawMessage{}
			})

			It("should update it to an empty array", func() {
				Expect(lrpClient.UpdateCallCount()).To(Equal(1))
				_, lrp := lrpClient.UpdateArgsForCall(0)
				Expect(lrp.AppURIs).To(BeEmpty())
			})
		})

		Context("when the app does not exist", func() {
			BeforeEach(func() {
				lrpClient.GetReturns(nil, errors.New("app does not exist"))
			})

			It("should not submit anything to be updated", func() {
				Expect(lrpClient.UpdateCallCount()).To(Equal(0))
			})

			It("should propagate the error", func() {
				Expect(err).To(MatchError(ContainSubstring("failed to get app")))
			})
		})
	})

	Describe("Get an App", func() {
		var (
			lrp        *api.LRP
			identifier api.LRPIdentifier
		)

		JustBeforeEach(func() {
			identifier = api.LRPIdentifier{
				GUID:    "guid_1234",
				Version: "version_1234",
			}
		})

		Context("when the app exists", func() {
			BeforeEach(func() {
				lrp = &api.LRP{
					TargetInstances: 5,
					LastUpdated:     "1234.5",
					AppURIs: []api.Route{
						{Hostname: "route1.io", Port: 6666},
						{Hostname: "route2.io", Port: 9999},
					},
					Image: "the/image",
				}

				lrpClient.GetReturns(lrp, nil)
			})

			It("should use the LRPClient to get the lrp", func() {
				_, err = lrpBifrost.GetApp(context.Background(), identifier)
				Expect(err).NotTo(HaveOccurred())
				Expect(lrpClient.GetCallCount()).To(Equal(1))
				_, actualID := lrpClient.GetArgsForCall(0)
				Expect(actualID).To(Equal(identifier))
			})

			It("should return a DesiredLRP", func() {
				desiredLRP, _ := lrpBifrost.GetApp(context.Background(), identifier)
				Expect(desiredLRP).ToNot(BeNil())
				Expect(desiredLRP.ProcessGUID).To(Equal("guid_1234-version_1234"))
				Expect(desiredLRP.Instances).To(Equal(int32(5)))
				Expect(desiredLRP.Annotation).To(Equal("1234.5"))
				Expect(desiredLRP.Routes).To(HaveKeyWithValue("cf-router", json.RawMessage(`[{"hostname":"route1.io","port":6666},{"hostname":"route2.io","port":9999}]`)))
				Expect(desiredLRP.Image).To(Equal("the/image"))
			})
		})

		Context("when the app does not exist", func() {
			BeforeEach(func() {
				lrpClient.GetReturns(nil, errors.New("Failed to get LRP"))
			})

			It("should return an error", func() {
				_, getAppErr := lrpBifrost.GetApp(context.Background(), identifier)
				Expect(getAppErr).To(MatchError(ContainSubstring("failed to get app")))
			})
		})
	})

	Describe("Stop an app", func() {
		JustBeforeEach(func() {
			err = lrpBifrost.Stop(context.Background(), api.LRPIdentifier{GUID: "guid_1234", Version: "version_1234"})
		})

		It("should not return an error", func() {
			Expect(err).ToNot(HaveOccurred())
		})

		It("should call the lrpClient with the expected guid", func() {
			_, identifier := lrpClient.StopArgsForCall(0)
			Expect(identifier.GUID).To(Equal("guid_1234"))
			Expect(identifier.Version).To(Equal("version_1234"))
		})

		Context("when LRPClient's stop fails", func() {
			BeforeEach(func() {
				lrpClient.StopReturns(errors.New("failed-to-stop"))
			})

			It("returns an error", func() {
				Expect(err).To(MatchError(ContainSubstring("failed to stop app")))
			})
		})
	})

	Describe("Stop an app instance", func() {
		JustBeforeEach(func() {
			err = lrpBifrost.StopInstance(context.Background(), api.LRPIdentifier{GUID: "guid_1234", Version: "version_1234"}, "1")
		})

		It("should not return an error", func() {
			Expect(err).ToNot(HaveOccurred())
		})

		It("should call the LRPClient with the expected guid and index", func() {
			_, identifier, index := lrpClient.StopInstanceArgsForCall(0)
			Expect(identifier.GUID).To(Equal("guid_1234"))
			Expect(identifier.Version).To(Equal("version_1234"))
			Expect(index).To(Equal(uint(1)))
		})

		Context("when LRPClient's stop instance fails", func() {
			BeforeEach(func() {
				lrpClient.StopInstanceReturns(errors.New("failed-to-stop"))
			})

			It("returns a meaningful error", func() {
				Expect(err).To(MatchError(ContainSubstring("failed to stop instance")))
			})
		})
	})

	Describe("Get all instances of an app", func() {
		var (
			instances    []*cf.Instance
			apiInstances []*api.Instance
		)

		BeforeEach(func() {
			apiInstances = []*api.Instance{
				{Index: "0", Since: 123, State: api.RunningState},
				{Index: "1", Since: 345, State: api.CrashedState},
				{Index: "2", Since: 678, State: api.ErrorState, PlacementError: "this is not the place"},
			}

			lrpClient.GetInstancesReturns(apiInstances, nil)
		})

		JustBeforeEach(func() {
			instances, err = lrpBifrost.GetInstances(context.Background(), api.LRPIdentifier{GUID: "guid_1234", Version: "version_1234"})
		})

		It("should get the app instances from lrpClient", func() {
			Expect(lrpClient.GetInstancesCallCount()).To(Equal(1))
			_, identifier := lrpClient.GetInstancesArgsForCall(0)
			Expect(identifier.GUID).To(Equal("guid_1234"))
			Expect(identifier.Version).To(Equal("version_1234"))
		})

		It("should not return an error", func() {
			Expect(err).ToNot(HaveOccurred())
		})

		It("should return all running instances", func() {
			Expect(instances).To(Equal([]*cf.Instance{
				{Index: "0", Since: 123, State: api.RunningState},
				{Index: "1", Since: 345, State: api.CrashedState},
				{Index: "2", Since: 678, State: api.ErrorState, PlacementError: "this is not the place"},
			}))
		})

		Context("when the app does not exist", func() {
			BeforeEach(func() {
				lrpClient.GetInstancesReturns([]*api.Instance{}, errors.New("not found"))
			})

			It("returns an error", func() {
				Expect(err).To(MatchError(ContainSubstring("failed to get instances for app")))
			})
		})
	})
})

package eats_test

import (
	"encoding/json"
	"net/http"

	"code.cloudfoundry.org/eirini/models/cf"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gstruct"
)

var _ = Describe("Apps", func() {
	Describe("Desiring an app", func() {

		var desireResp *http.Response

		JustBeforeEach(func() {
			desireResp = desireApp("the-app-guid", "the-version")
		})

		It("succeeds", func() {
			Expect(desireResp.StatusCode).To(Equal(http.StatusAccepted))
		})

		It("deploys the LRP", func() {
			lrp, err := getLRP("the-app-guid", "the-version")
			Expect(err).NotTo(HaveOccurred())
			Expect(lrp.ProcessGUID).To(Equal("the-app-guid-the-version"))
		})

		When("the app already exist", func() {
			It("returns 400", func() {
				resp := desireApp("the-app-guid", "the-version")
				Expect(resp.StatusCode).To(Equal(http.StatusBadRequest))
			})
		})
	})

	Describe("Update an app", func() {
		BeforeEach(func() {
			desireApp("the-app-guid", "the-version")
		})

		It("successfully updates the app", func() {
			updatedRoutes := []routeInfo{{Hostname: "updated-host", Port: 4321}}
			updateResp := updateApp("the-app-guid", "the-version", 2, "333333", updatedRoutes)
			Expect(updateResp.StatusCode).To(Equal(http.StatusOK))
			lrp, err := getLRP("the-app-guid", "the-version")
			Expect(err).NotTo(HaveOccurred())
			Expect(lrp.Instances).To(BeNumerically("==", 2))
			Expect(lrp.Annotation).To(Equal("333333"))

			Expect(lrp.Routes).To(SatisfyAll(
				HaveLen(1),
				HaveKeyWithValue("cf-router", marshalRoutes(updatedRoutes))),
			)
		})
	})

	Describe("Listing apps", func() {
		JustBeforeEach(func() {
			firstLrp := createLrpRequest("the-first-app-guid", "v1")
			firstLrp.NumInstances = 2
			firstLrp.LastUpdated = "111111"
			desireLRP(firstLrp)

			secondLrp := createLrpRequest("the-second-app-guid", "v1")
			secondLrp.LastUpdated = "222222"
			desireLRP(secondLrp)
		})

		It("successfully lists all LRPs", func() {
			lrps, err := getLRPs()
			Expect(err).NotTo(HaveOccurred())

			lrpsAnnotations := make(map[string]string)
			for _, lrp := range lrps {
				lrpsAnnotations[lrp.DesiredLRPKey.ProcessGUID] = lrp.Annotation
			}

			Expect(lrpsAnnotations).To(SatisfyAll(HaveLen(2), HaveKey("the-first-app-guid-v1"), HaveKey("the-second-app-guid-v1")))
			Expect(lrpsAnnotations["the-first-app-guid-v1"]).To(Equal("111111"))
			Expect(lrpsAnnotations["the-second-app-guid-v1"]).To(Equal("222222"))
		})
	})

	Describe("Get an app", func() {
		JustBeforeEach(func() {
			desireApp("the-app-guid", "v1")
		})

		It("successfully returns the LRP", func() {
			lrp, err := getLRP("the-app-guid", "v1")
			Expect(err).NotTo(HaveOccurred())
			Expect(lrp.ProcessGUID).To(Equal("the-app-guid-v1"))
			Expect(lrp.Instances).To(Equal(int32(1)))
		})

		When("the app doesn't exist", func() {
			It("returns a 404", func() {
				_, err := getLRP("does-not-exist", "v1")
				Expect(err).To(MatchError("404 Not Found"))
			})
		})
	})

	Describe("Stop an app", func() {
		BeforeEach(func() {
			desireApp("the-app-guid", "v1")
		})

		It("successfully stops the app", func() {
			_, err := stopLRP("the-app-guid", "v1")
			Expect(err).NotTo(HaveOccurred())
			_, err = getLRP("the-app-guid", "v1")
			Expect(err).To(MatchError("404 Not Found"))
		})

		When("the app doesn't exist", func() {
			It("returns a 404", func() {
				response, err := stopLRP("does-not-exist", "v1")
				Expect(err).NotTo(HaveOccurred())
				Expect(response.StatusCode).To(Equal(http.StatusNotFound))
			})
		})
	})

	Describe("Stop an app instance", func() {
		BeforeEach(func() {
			desireAppWithInstances("the-app-guid", "v1", 3)
			Eventually(func() []*cf.Instance {
				return getRunningInstances("the-app-guid", "v1")
			}, "20s").Should(HaveLen(3))
		})

		It("successfully stops the instance", func() {
			_, err := stopLRPInstance("the-app-guid", "v1", 1)
			Expect(err).NotTo(HaveOccurred())

			Eventually(func() []*cf.Instance {
				return getRunningInstances("the-app-guid", "v1")
			}, "10s").Should(ConsistOf(
				gstruct.PointTo(gstruct.MatchFields(gstruct.IgnoreExtras, gstruct.Fields{
					"Index": Equal(0),
					"State": Equal("RUNNING"),
					"Since": BeNumerically(">", 0),
				})),
				gstruct.PointTo(gstruct.MatchFields(gstruct.IgnoreExtras, gstruct.Fields{
					"Index": Equal(2),
					"State": Equal("RUNNING"),
					"Since": BeNumerically(">", 0),
				})),
			))
		})

		When("the app does not exist", func() {
			It("should return 404", func() {
				resp, err := stopLRPInstance("does-not-exist", "v1", 1)
				Expect(err).NotTo(HaveOccurred())
				Expect(resp.StatusCode).To(Equal(http.StatusNotFound))
			})
		})

		When("the app instance does not exist", func() {
			It("should return 400", func() {
				resp, err := stopLRPInstance("the-app-guid", "v1", 99)
				Expect(err).NotTo(HaveOccurred())
				Expect(resp.StatusCode).To(Equal(http.StatusBadRequest))
			})
		})

		When("the app instance is a negative number", func() {
			It("should return 400", func() {
				resp, err := stopLRPInstance("the-app-guid", "v1", -1)
				Expect(err).NotTo(HaveOccurred())
				Expect(resp.StatusCode).To(Equal(http.StatusBadRequest))
			})
		})
	})

	Describe("Get instances", func() {
		JustBeforeEach(func() {
			desireAppWithInstances("the-app-guid", "v1", 3)
		})

		It("returns the app instances", func() {
			var resp *cf.GetInstancesResponse
			Eventually(func() []*cf.Instance {
				var err error
				resp, err = getInstances("the-app-guid", "v1")
				Expect(err).NotTo(HaveOccurred())

				return resp.Instances
			}, "20s").Should(ConsistOf(
				gstruct.PointTo(gstruct.MatchFields(gstruct.IgnoreExtras, gstruct.Fields{
					"Index": Equal(0),
					"State": Equal("RUNNING"),
					"Since": BeNumerically(">", 0),
				})),
				gstruct.PointTo(gstruct.MatchFields(gstruct.IgnoreExtras, gstruct.Fields{
					"Index": Equal(1),
					"State": Equal("RUNNING"),
					"Since": BeNumerically(">", 0),
				})),
				gstruct.PointTo(gstruct.MatchFields(gstruct.IgnoreExtras, gstruct.Fields{
					"Index": Equal(2),
					"State": Equal("RUNNING"),
					"Since": BeNumerically(">", 0),
				})),
			))
		})

		When("the app doesn't exist", func() {
			It("returns a 404", func() {
				resp, err := getInstances("does-not-exist", "v1")
				Expect(err).To(MatchError("404 Not Found"))
				Expect(resp.Error).To(Equal("failed to get instances for app: not found"))
			})
		})
	})
})

func desireApp(appGUID, version string) *http.Response { // nolint:unparam
	return desireAppWithInstances(appGUID, version, 1)
}

func desireAppWithInstances(appGUID, version string, instances int) *http.Response {
	lrp := createLrpRequest(appGUID, version)
	lrp.NumInstances = instances
	return desireLRP(lrp)
}

func createLrpRequest(appGUID, version string) cf.DesireLRPRequest {
	return cf.DesireLRPRequest{
		GUID:         appGUID,
		Version:      version,
		NumInstances: 1,
		Ports:        []int32{8080},
		Lifecycle: cf.Lifecycle{
			DockerLifecycle: &cf.DockerLifecycle{
				Image: "eirini/dorini",
			},
		},
	}
}

func updateApp(appGUID, version string, instances int, annotation string, routes []routeInfo) *http.Response {
	resp, err := updateLRP(cf.UpdateDesiredLRPRequest{
		GUID:    appGUID,
		Version: version,
		Update: cf.DesiredLRPUpdate{
			Instances: instances,
			Routes: map[string]json.RawMessage{
				"cf-router": marshalRoutes(routes),
			},
			Annotation: annotation,
		},
	})
	Expect(err).NotTo(HaveOccurred())
	return resp
}

func getRunningInstances(appGUID, version string) []*cf.Instance {
	instancesResp, err := getInstances(appGUID, version)
	Expect(err).NotTo(HaveOccurred())

	runningInstances := []*cf.Instance{}
	for _, i := range instancesResp.Instances {
		if i.State == "RUNNING" {
			runningInstances = append(runningInstances, i)
		}
	}
	return runningInstances
}

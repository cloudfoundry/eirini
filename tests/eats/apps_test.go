package eats_test

import (
	"encoding/json"
	"fmt"
	"net/http"

	"code.cloudfoundry.org/eirini/models/cf"
	"code.cloudfoundry.org/eirini/tests"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gstruct"
)

var _ = Describe("Apps [needs-logs-for: eirini-api]", func() {
	var (
		namespace      string
		lrpGUID        string
		lrpVersion     string
		lrpProcessGUID string
		appServiceName string
	)

	BeforeEach(func() {
		namespace = fixture.Namespace
		lrpGUID = tests.GenerateGUID()
		lrpVersion = tests.GenerateGUID()
		lrpProcessGUID = processGUID(lrpGUID, lrpVersion)
	})

	Describe("Desiring an app", func() {
		var desireRespStatusCode int

		JustBeforeEach(func() {
			desireRespStatusCode = desireApp(lrpGUID, lrpVersion, namespace)
			appServiceName = tests.ExposeAsService(fixture.Clientset, fixture.Namespace, lrpGUID, 8080)
		})

		AfterEach(func() {
			_, err := stopLRP(lrpGUID, lrpVersion)
			Expect(err).NotTo(HaveOccurred())
		})

		It("succeeds", func() {
			Expect(desireRespStatusCode).To(Equal(http.StatusAccepted))
		})

		It("runs the application", func() {
			Eventually(tests.RequestServiceFn(fixture.Namespace, appServiceName, 8080, "/")).Should(ContainSubstring("Hi, I'm not Dora!"))
		})

		When("the app already exist", func() {
			It("returns 202", func() {
				respStatusCode := desireApp(lrpGUID, lrpVersion, namespace)
				Expect(respStatusCode).To(Equal(http.StatusAccepted))
			})
		})

		When("the app has private docker image", func() {
			var (
				privateImageAppGUID    string
				privateImageAppVersion string
				appServiceName         string
			)

			BeforeEach(func() {
				privateImageAppGUID = tests.GenerateGUID()
				privateImageAppVersion = tests.GenerateGUID()
			})

			It("creates a running application", func() {
				lrp := createLrpRequest(privateImageAppGUID, privateImageAppVersion)
				lrp.Namespace = namespace
				lrp.Lifecycle = cf.Lifecycle{
					DockerLifecycle: &cf.DockerLifecycle{
						Image:            "eirini/dorini",
						RegistryUsername: "eiriniuser",
						RegistryPassword: tests.GetEiriniDockerHubPassword(),
					},
				}

				Expect(desireLRP(lrp)).To(Equal(http.StatusAccepted))
				appServiceName = tests.ExposeAsService(fixture.Clientset, fixture.Namespace, privateImageAppGUID, 8080)

				Eventually(tests.RequestServiceFn(fixture.Namespace, appServiceName, 8080, "/")).Should(ContainSubstring("Hi, I'm not Dora!"))
			})
		})
	})

	Describe("Update an app", func() {
		var (
			desiredLRPUpdate cf.DesiredLRPUpdate
			lrp              cf.DesiredLRP
		)

		BeforeEach(func() {
			desireApp(lrpGUID, lrpVersion, namespace)
		})

		JustBeforeEach(func() {
			updateResp, err := updateLRP(cf.UpdateDesiredLRPRequest{
				GUID:    lrpGUID,
				Version: lrpVersion,
				Update:  desiredLRPUpdate,
			})
			Expect(err).NotTo(HaveOccurred())
			Expect(updateResp.StatusCode).To(Equal(http.StatusOK))

			lrp, err = getLRP(lrpGUID, lrpVersion)
			Expect(err).NotTo(HaveOccurred())
		})

		AfterEach(func() {
			_, err := stopLRP(lrpGUID, lrpVersion)
			Expect(err).NotTo(HaveOccurred())
		})

		When("updating the instance number", func() {
			BeforeEach(func() {
				desiredLRPUpdate = cf.DesiredLRPUpdate{
					Instances: 2,
				}
			})

			It("successfully updates the LRP", func() {
				Expect(lrp.Instances).To(BeNumerically("==", 2))
			})
		})

		When("updating the routes", func() {
			var updatedRoutes []tests.RouteInfo

			BeforeEach(func() {
				updatedRoutes = []tests.RouteInfo{{Hostname: "updated-host", Port: 4321}}
				desiredLRPUpdate = cf.DesiredLRPUpdate{
					Routes: map[string]json.RawMessage{
						"cf-router": tests.MarshalRoutes(updatedRoutes),
					},
				}
			})

			It("successfully updates the LRP", func() {
				Expect(lrp.Routes).To(SatisfyAll(
					HaveLen(1),
					HaveKeyWithValue("cf-router", tests.MarshalRoutes(updatedRoutes))),
				)
			})
		})

		When("updating the annotation", func() {
			BeforeEach(func() {
				desiredLRPUpdate = cf.DesiredLRPUpdate{
					Annotation: "333333",
				}
			})

			It("successfully updates the LRP", func() {
				Expect(lrp.Annotation).To(Equal("333333"))
			})
		})

		When("updating the image", func() {
			BeforeEach(func() {
				desiredLRPUpdate = cf.DesiredLRPUpdate{
					Image: "new/image",
				}
			})

			It("successfully updates the LRP", func() {
				Expect(lrp.Image).To(Equal("new/image"))
			})
		})
	})

	Describe("Listing apps", func() {
		var (
			anotherLrpGUID    string
			anotherLrpVersion string
		)

		JustBeforeEach(func() {
			anotherLrpGUID = tests.GenerateGUID()
			anotherLrpVersion = tests.GenerateGUID()

			firstLrp := createLrpRequest(lrpGUID, lrpVersion)
			firstLrp.NumInstances = 2
			firstLrp.LastUpdated = "111111"
			firstLrp.Namespace = namespace
			desireLRP(firstLrp)

			secondLrp := createLrpRequest(anotherLrpGUID, anotherLrpVersion)
			secondLrp.LastUpdated = "222222"
			secondLrp.Namespace = namespace
			desireLRP(secondLrp)
		})

		AfterEach(func() {
			_, err := stopLRP(lrpGUID, lrpVersion)
			Expect(err).NotTo(HaveOccurred())

			_, err = stopLRP(anotherLrpGUID, anotherLrpVersion)
			Expect(err).NotTo(HaveOccurred())
		})

		It("successfully lists all LRPs", func() {
			lrps, err := getLRPs()
			Expect(err).NotTo(HaveOccurred())

			lrpsAnnotations := make(map[string]string)
			for _, lrp := range lrps {
				lrpsAnnotations[lrp.DesiredLRPKey.ProcessGUID] = lrp.Annotation
			}

			anotherProcessGUID := processGUID(anotherLrpGUID, anotherLrpVersion)

			Expect(lrpsAnnotations).To(SatisfyAll(HaveKey(lrpProcessGUID), HaveKey(anotherProcessGUID)))
			Expect(lrpsAnnotations[lrpProcessGUID]).To(Equal("111111"))
			Expect(lrpsAnnotations[anotherProcessGUID]).To(Equal("222222"))
		})
	})

	Describe("Get an app", func() {
		JustBeforeEach(func() {
			desireApp(lrpGUID, lrpVersion, namespace)
		})

		AfterEach(func() {
			_, err := stopLRP(lrpGUID, lrpVersion)
			Expect(err).NotTo(HaveOccurred())
		})

		It("successfully returns the LRP", func() {
			lrp, err := getLRP(lrpGUID, lrpVersion)
			Expect(err).NotTo(HaveOccurred())
			Expect(lrp.ProcessGUID).To(Equal(lrpProcessGUID))
			Expect(lrp.Instances).To(Equal(int32(1)))
		})

		When("the app doesn't exist", func() {
			It("returns a 404", func() {
				_, err := getLRP("does-not-exist", lrpVersion)
				Expect(err).To(MatchError("404 Not Found"))
			})
		})
	})

	Describe("Stop an app", func() {
		var (
			stopErr      error
			stopResponse *http.Response
		)

		BeforeEach(func() {
			desireApp(lrpGUID, lrpVersion, namespace)
		})

		JustBeforeEach(func() {
			stopResponse, stopErr = stopLRP(lrpGUID, lrpVersion)
		})

		It("succeeds", func() {
			Expect(stopErr).NotTo(HaveOccurred())
		})

		It("deletes the LRP", func() {
			_, err := getLRP(lrpGUID, lrpVersion)
			Expect(err).To(MatchError("404 Not Found"))
		})

		When("the app doesn't exist", func() {
			BeforeEach(func() {
				lrpGUID = "does-not-exist"
			})

			It("succeeds", func() {
				Expect(stopErr).NotTo(HaveOccurred())
				Expect(stopResponse.StatusCode).To(Equal(http.StatusOK))
			})
		})
	})

	Describe("Stop an app instance", func() {
		var (
			stopResponse *http.Response
			stopErr      error
			instanceID   string
		)

		BeforeEach(func() {
			desireAppWithInstances(lrpGUID, lrpVersion, namespace, 3)
			Eventually(func() []*cf.Instance {
				return getRunningInstances(lrpGUID, lrpVersion)
			}).Should(HaveLen(3))
			instanceID = getRunningInstances(lrpGUID, lrpVersion)[0].Index
		})

		JustBeforeEach(func() {
			stopResponse, stopErr = stopLRPInstance(lrpGUID, lrpVersion, instanceID)
		})

		It("succeeds", func() {
			Expect(stopErr).NotTo(HaveOccurred())
		})

		It("successfully stops the instance", func() {
			Eventually(func() []*cf.Instance {
				return getRunningInstances(lrpGUID, lrpVersion)
			}).Should(ConsistOf(
				gstruct.PointTo(gstruct.MatchFields(gstruct.IgnoreExtras, gstruct.Fields{
					"Index": HaveLen(5),
					"State": Equal("RUNNING"),
					"Since": BeNumerically(">", 0),
				})),
				gstruct.PointTo(gstruct.MatchFields(gstruct.IgnoreExtras, gstruct.Fields{
					"Index": HaveLen(5),
					"State": Equal("RUNNING"),
					"Since": BeNumerically(">", 0),
				})),
			))
		})

		When("the app does not exist", func() {
			BeforeEach(func() {
				lrpGUID = "does-not-exist"
			})
			It("succeeds", func() {
				Expect(stopErr).NotTo(HaveOccurred())
				Expect(stopResponse.StatusCode).To(Equal(http.StatusOK))
			})
		})

		When("the app instance does not exist", func() {
			BeforeEach(func() {
				instanceID = "none"
			})

			It("should return 400", func() {
				Expect(stopErr).NotTo(HaveOccurred())
				Expect(stopResponse.StatusCode).To(Equal(http.StatusBadRequest))
			})
		})

		// When("the app instance is a negative number", func() {
		// 	BeforeEach(func() {
		// 		instanceID = -1
		// 	})

		// 	It("should return 400", func() {
		// 		Expect(stopErr).NotTo(HaveOccurred())
		// 		Expect(stopResponse.StatusCode).To(Equal(http.StatusBadRequest))
		// 	})
		// })
	})

	Describe("Get instances", func() {
		var (
			getInstanceErr       error
			getInstancesResponse *cf.GetInstancesResponse
		)

		JustBeforeEach(func() {
			desireAppWithInstances(lrpGUID, lrpVersion, namespace, 3)
			getInstancesResponse, getInstanceErr = getInstances(lrpGUID, lrpVersion)
		})

		It("succeeds", func() {
			Expect(getInstanceErr).NotTo(HaveOccurred())
		})

		It("returns the app instances", func() {
			var resp *cf.GetInstancesResponse
			Eventually(func() []*cf.Instance {
				var err error
				resp, err = getInstances(lrpGUID, lrpVersion)
				Expect(err).NotTo(HaveOccurred())

				return resp.Instances
			}).Should(ConsistOf(
				gstruct.PointTo(gstruct.MatchFields(gstruct.IgnoreExtras, gstruct.Fields{
					"Index": HaveLen(5),
					"State": Equal("RUNNING"),
					"Since": BeNumerically(">", 0),
				})),
				gstruct.PointTo(gstruct.MatchFields(gstruct.IgnoreExtras, gstruct.Fields{
					"Index": HaveLen(5),
					"State": Equal("RUNNING"),
					"Since": BeNumerically(">", 0),
				})),
				gstruct.PointTo(gstruct.MatchFields(gstruct.IgnoreExtras, gstruct.Fields{
					"Index": HaveLen(5),
					"State": Equal("RUNNING"),
					"Since": BeNumerically(">", 0),
				})),
			))
		})

		When("the app doesn't exist", func() {
			JustBeforeEach(func() {
				getInstancesResponse, getInstanceErr = getInstances("does-not-exist", lrpVersion)
			})

			It("returns a 404", func() {
				Expect(getInstanceErr).To(MatchError("404 Not Found"))
				Expect(getInstancesResponse.Error).To(Equal("failed to get instances for app: not found"))
			})
		})
	})
})

func desireAppWithInstances(appGUID, version, namespace string, instances int) {
	lrp := createLrpRequest(appGUID, version)
	lrp.NumInstances = instances
	lrp.Namespace = namespace

	desireLRP(lrp)
}

func desireApp(appGUID, version, namespace string) int {
	lrp := createLrpRequest(appGUID, version)
	lrp.Namespace = namespace

	return desireLRP(lrp)
}

func createLrpRequest(appGUID, version string) cf.DesireLRPRequest {
	return cf.DesireLRPRequest{
		GUID:         appGUID,
		Version:      version,
		NumInstances: 1,
		Ports:        []int32{8080},
		DiskMB:       100,
		Lifecycle: cf.Lifecycle{
			DockerLifecycle: &cf.DockerLifecycle{
				Image: "eirini/dorini",
			},
		},
	}
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

func processGUID(guid, version string) string {
	return fmt.Sprintf("%s-%s", guid, version)
}

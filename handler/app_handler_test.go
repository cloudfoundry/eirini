package handler_test

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"strings"

	"code.cloudfoundry.org/eirini"
	. "code.cloudfoundry.org/eirini/handler"
	"code.cloudfoundry.org/eirini/handler/handlerfakes"
	"code.cloudfoundry.org/eirini/models/cf"
	"code.cloudfoundry.org/lager/lagertest"
	"github.com/julienschmidt/httprouter"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/pkg/errors"
)

var _ = Describe("AppHandler", func() {
	var (
		lrpBifrost     *handlerfakes.FakeLRPBifrost
		stagingBifrost *handlerfakes.FakeStagingBifrost
		lager          *lagertest.TestLogger
		ts             *httptest.Server
	)

	BeforeEach(func() {
		lrpBifrost = new(handlerfakes.FakeLRPBifrost)
		stagingBifrost = new(handlerfakes.FakeStagingBifrost)
		lager = lagertest.NewTestLogger("app-handler-test")
		ts = httptest.NewServer(New(lrpBifrost, stagingBifrost, nil, nil, lager))
	})

	AfterEach(func() {
		ts.Close()
	})

	findLog := func(message, guid string) func() {
		return func() {
			logs := lager.Logs()
			Expect(logs).NotTo(HaveLen(0))
			found := false
			for _, log := range logs {
				if log.Message == message {
					Expect(log.Data).To(HaveKeyWithValue("guid", guid))
					found = true
				}
			}
			Expect(found).To(BeTrue(), fmt.Sprintf("haven't received %s message", message))
		}
	}

	Context("Desire an app", func() {
		var (
			path     string
			body     string
			response *http.Response
		)

		BeforeEach(func() {
			path = "/apps/myguid"
			body = `{
				"guid": "guid",
				"process_guid" : "myguid",
				"process_type": "web",
				"space_guid": "space-guid",
				"space_name": "space-name",
				"app_name": "app-name",
				"app_guid": "app-guid",
				"organization_guid": "org-guid",
				"organization_name": "org-name",
				"version": "version",
				"placement_tags": ["place-1", "place-2"],
				"egress_rules": ["raw message"],
				"lifecycle": {
					"buildpack_lifecycle": {
						"start_command": "./start"
					}
				},
				"environment": { "env_var": "env_var_value" },
				"instances": 5,
				"memory_mb": 123,
				"cpu_weight": 10,
				"disk_mb": 256,
				"last_updated":"1529073295.9",
				"health_check_type":"http",
				"health_check_http_endpoint":"/healthz",
				"health_check_timeout_ms":400,
				"start_timeout_ms": 320,
				"volume_mounts": [
					{
						"mount_dir": "/var/vcap/data/e1df89b4-33de-4d72-b471-5495222177c8",
						"volume_id":"vol1"
					}
				],
				"ports":[8080,7777],
				"user_defined_annotations": {
						"prometheus.io/scrape": "scrape"
				}
			}`
		})

		JustBeforeEach(func() {
			req, err := http.NewRequest("PUT", ts.URL+path, bytes.NewReader([]byte(body)))
			Expect(err).NotTo(HaveOccurred())

			client := &http.Client{}
			response, err = client.Do(req)
			Expect(err).ToNot(HaveOccurred())
		})

		It("should call the bifrost with the desired LRPs request from Cloud Controller", func() {
			expectedRequest := cf.DesireLRPRequest{
				GUID:             "guid",
				ProcessGUID:      "myguid",
				Version:          "version",
				ProcessType:      "web",
				AppGUID:          "app-guid",
				AppName:          "app-name",
				EgressRules:      []json.RawMessage{[]byte("\"raw message\"")},
				SpaceGUID:        "space-guid",
				SpaceName:        "space-name",
				OrganizationGUID: "org-guid",
				OrganizationName: "org-name",
				PlacementTags:    []string{"place-1", "place-2"},
				Lifecycle: cf.Lifecycle{
					BuildpackLifecycle: &cf.BuildpackLifecycle{
						StartCommand: "./start",
					},
				},
				Environment:             map[string]string{"env_var": "env_var_value"},
				NumInstances:            5,
				MemoryMB:                123,
				DiskMB:                  256,
				CPUWeight:               10,
				LastUpdated:             "1529073295.9",
				HealthCheckType:         "http",
				HealthCheckHTTPEndpoint: "/healthz",
				HealthCheckTimeoutMs:    400,
				StartTimeoutMs:          320,
				Ports:                   []int32{8080, 7777},
				VolumeMounts: []cf.VolumeMount{
					{
						MountDir: "/var/vcap/data/e1df89b4-33de-4d72-b471-5495222177c8",
						VolumeID: "vol1",
					},
				},
				LRP: body,
				UserDefinedAnnotations: map[string]string{
					"prometheus.io/scrape": "scrape",
				},
			}

			Expect(lrpBifrost.TransferCallCount()).To(Equal(1))
			_, request := lrpBifrost.TransferArgsForCall(0)
			Expect(request).To(Equal(expectedRequest))
		})

		Context("When Bifrost fails to handle desire request", func() {
			BeforeEach(func() {
				lrpBifrost.TransferReturns(errors.New("aaargh"))
			})

			It("should return BadRequest status", func() {
				Expect(response.StatusCode).To(Equal(http.StatusBadRequest))
			})

			It("should provide a helpful log message", findLog("app-handler-test.desire-app.bifrost-failed", "myguid"))
		})

		Context("when the body is empty", func() {
			BeforeEach(func() {
				body = ""
			})

			It("should return a 400 Bad Request HTTP status code", func() {
				Expect(response.StatusCode).To(Equal(http.StatusBadRequest))
			})

			It("should provide a helpful log message", findLog("app-handler-test.desire-app.request-body-decoding-failed", "myguid"))

			It("should not update the app", func() {
				Expect(lrpBifrost.TransferCallCount()).To(Equal(0))
			})
		})
	})

	Context("List Apps", func() {
		var (
			appHandler           *App
			responseRecorder     *httptest.ResponseRecorder
			expectedJSONResponse string
			schedInfos           []cf.DesiredLRPSchedulingInfo
		)

		BeforeEach(func() {
			schedInfos = createSchedulingInfos()
			lrpBifrost.ListReturns(schedInfos, nil)
		})

		JustBeforeEach(func() {
			req, err := http.NewRequest("", "/apps", nil)
			Expect(err).ToNot(HaveOccurred())
			responseRecorder = httptest.NewRecorder()
			appHandler = NewAppHandler(lrpBifrost, lager)
			appHandler.List(responseRecorder, req, httprouter.Params{})
			expectedResponse := cf.DesiredLRPSchedulingInfosResponse{
				DesiredLrpSchedulingInfos: schedInfos,
			}
			expectedJSONResponseBytes, err := json.Marshal(expectedResponse)
			Expect(err).ToNot(HaveOccurred())
			expectedJSONResponse = string(expectedJSONResponseBytes)
		})

		Context("When there are existing apps", func() {
			It("should list all DesiredLRPSchedulingInfos as JSON in the response body", func() {
				Expect(responseRecorder.Code).To(Equal(http.StatusOK))

				body, err := readBody(responseRecorder.Body)
				Expect(err).ToNot(HaveOccurred())

				Expect(strings.Trim(string(body), "\n")).To(Equal(expectedJSONResponse))
			})
		})

		Context("When there are no existing apps", func() {
			BeforeEach(func() {
				schedInfos = []cf.DesiredLRPSchedulingInfo{}
				lrpBifrost.ListReturns(schedInfos, nil)
			})

			It("should return an empty list of DesiredLRPSchedulingInfo", func() {
				Expect(responseRecorder.Code).To(Equal(http.StatusOK))

				body, err := readBody(responseRecorder.Body)
				Expect(err).ToNot(HaveOccurred())

				Expect(strings.Trim(string(body), "\n")).To(Equal(expectedJSONResponse))
			})
		})

		Context("When bifrost returns an error", func() {
			BeforeEach(func() {
				lrpBifrost.ListReturns(nil, errors.New("something-went-wrong"))
			})

			It("should return BadRequest status", func() {
				Expect(responseRecorder.Code).To(Equal(http.StatusInternalServerError))
			})

			It("should provide a helpful log message", func() {
				logs := lager.Logs()
				Expect(logs).NotTo(HaveLen(0))
				found := false
				for _, log := range logs {
					if log.Message == "app-handler-test.list-apps.bifrost-failed" {
						found = true
					}
				}
				Expect(found).To(BeTrue(), "haven't received app-handler-test.list-apps.bifrost-failed message")
			})
		})

		Context("When there are no apps", func() {
			BeforeEach(func() {
				lrpBifrost.ListReturns([]cf.DesiredLRPSchedulingInfo{}, nil)
			})

			It("returns an empty non-nil desired_lrp_scheduling_infos array", func() {
				body, err := readBody(responseRecorder.Body)
				Expect(err).ToNot(HaveOccurred())

				Expect(strings.Trim(string(body), "\n")).To(Equal(`{"desired_lrp_scheduling_infos":[]}`))
			})
		})
	})

	Context("Get an app", func() {
		var (
			path       string
			response   *http.Response
			desiredLRP cf.DesiredLRP
		)

		BeforeEach(func() {
			path = "/apps/guid_1234/version_1234"
		})

		JustBeforeEach(func() {
			req, err := http.NewRequest("GET", ts.URL+path, nil)
			Expect(err).NotTo(HaveOccurred())

			client := &http.Client{}
			response, err = client.Do(req)
			Expect(err).ToNot(HaveOccurred())
		})

		It("should use the bifrost to get the app", func() {
			Expect(lrpBifrost.GetAppCallCount()).To(Equal(1))
			_, identifier := lrpBifrost.GetAppArgsForCall(0)
			Expect(identifier.GUID).To(Equal("guid_1234"))
			Expect(identifier.Version).To(Equal("version_1234"))
		})

		Context("when the app exists", func() {
			BeforeEach(func() {
				desiredLRP = cf.DesiredLRP{
					ProcessGUID: "guid_1234-version_1234",
					Instances:   5,
				}
				lrpBifrost.GetAppReturns(desiredLRP, nil)
			})

			It("should return a 200 HTTP status code", func() {
				Expect(response.StatusCode).To(Equal(http.StatusOK))
			})

			It("should return the DesiredLRP in the response body", func() {
				var getLRPResponse cf.DesiredLRPResponse
				err := json.NewDecoder(response.Body).Decode(&getLRPResponse)
				Expect(err).ToNot(HaveOccurred())

				actualLRP := getLRPResponse.DesiredLRP
				Expect(actualLRP.ProcessGUID).To(Equal("guid_1234-version_1234"))
				Expect(actualLRP.Instances).To(Equal(int32(5)))
			})
		})

		Context("when the app does not exist", func() {
			BeforeEach(func() {
				lrpBifrost.GetAppReturns(cf.DesiredLRP{}, errors.New("boom"))
			})

			It("should return a 404 HTTP status code", func() {
				Expect(response.StatusCode).To(Equal(http.StatusNotFound))
			})
		})
	})

	Context("Get Instances", func() {
		var (
			path     string
			response *http.Response
		)

		BeforeEach(func() {
			path = "/apps/guid_1234/version_1234/instances"

			instances := []*cf.Instance{
				{Index: 0, Since: 123, State: "RUNNING"},
				{Index: 1, Since: 456, State: "RUNNING"},
				{Index: 2, Since: 789, State: "UNCLAIMED", PlacementError: "this is not the place"},
			}
			lrpBifrost.GetInstancesReturns(instances, nil)
		})

		JustBeforeEach(func() {
			req, err := http.NewRequest("GET", ts.URL+path, nil)
			Expect(err).NotTo(HaveOccurred())

			client := &http.Client{}
			response, err = client.Do(req)
			Expect(err).ToNot(HaveOccurred())
		})

		It("should use bifrost to get all instances", func() {
			Expect(lrpBifrost.GetInstancesCallCount()).To(Equal(1))
			_, identifier := lrpBifrost.GetInstancesArgsForCall(0)
			Expect(identifier.GUID).To(Equal("guid_1234"))
			Expect(identifier.Version).To(Equal("version_1234"))
		})

		It("should return the instances in the response", func() {
			expectedResponse := `
				{
					"process_guid": "guid_1234-version_1234",
					"instances": [
						{
							"index": 0,
							"since": 123,
							"state": "RUNNING"
						},
						{
							"index": 1,
							"since": 456,
							"state": "RUNNING"
						},
						{
							"index": 2,
							"since": 789,
							"state": "UNCLAIMED",
							"placement_error": "this is not the place"
						}
					]
				}`
			body, err := ioutil.ReadAll(response.Body)
			Expect(err).ToNot(HaveOccurred())
			Expect(string(body)).To(MatchJSON(expectedResponse))
		})

		Context("when Bifrost returns an error", func() {
			BeforeEach(func() {
				lrpBifrost.GetInstancesReturns([]*cf.Instance{}, errors.New("failed to get instances"))
			})

			It("returns the error in the response", func() {
				expectedResponse := `
					{
						"error": "failed to get instances",
						"process_guid": "guid_1234-version_1234",
						"instances": []
					}`
				body, err := ioutil.ReadAll(response.Body)
				Expect(err).ToNot(HaveOccurred())
				Expect(string(body)).To(MatchJSON(expectedResponse))
			})

			It("should provide a helpful log message", findLog("app-handler-test.get-app-instances.bifrost-failed", "guid_1234"))
		})

		Context("when the app is not found", func() {
			BeforeEach(func() {
				lrpBifrost.GetInstancesReturns([]*cf.Instance{}, errors.Wrap(eirini.ErrNotFound, "failed to get instances"))
			})

			It("returns the error in the response", func() {
				expectedResponse := `
					{
						"error": "failed to get instances: not found",
						"process_guid": "guid_1234-version_1234",
						"instances": []
					}`
				Expect(response.StatusCode).To(Equal(http.StatusNotFound))
				body, err := ioutil.ReadAll(response.Body)
				Expect(err).ToNot(HaveOccurred())
				Expect(string(body)).To(MatchJSON(expectedResponse))
			})
		})
	})

	Context("Update an app", func() {
		var (
			path     string
			body     string
			response *http.Response
		)

		verifyResponseObject := func() {
			var responseObj cf.DesiredLRPLifecycleResponse
			err := json.NewDecoder(response.Body).Decode(&responseObj)
			Expect(err).ToNot(HaveOccurred())

			Expect(responseObj.Error.Message).ToNot(BeNil())
		}

		BeforeEach(func() {
			path = "/apps/myguid"
			body = `{"guid": "app-id", "version": "version-id", "update": {"instances": 5}}`
		})

		JustBeforeEach(func() {
			req, err := http.NewRequest("POST", ts.URL+path, bytes.NewReader([]byte(body)))
			Expect(err).NotTo(HaveOccurred())

			client := &http.Client{}
			response, err = client.Do(req)
			Expect(err).ToNot(HaveOccurred())
		})

		Context("when the update is successful", func() {
			BeforeEach(func() {
				lrpBifrost.UpdateReturns(nil)
			})

			It("should return a 200 HTTP stauts code", func() {
				Expect(response.StatusCode).To(Equal(http.StatusOK))
			})

			It("should translate the request", func() {
				Expect(lrpBifrost.UpdateCallCount()).To(Equal(1))
				_, request := lrpBifrost.UpdateArgsForCall(0)
				Expect(request.GUID).To(Equal("app-id"))
				Expect(request.Version).To(Equal("version-id"))
				Expect(request.Update.Instances).To(Equal(5))
			})
		})

		Context("when the json is invalid", func() {
			BeforeEach(func() {
				body = "{invalid.json"
			})

			It("should return a 400 Bad Request HTTP status code", func() {
				Expect(response.StatusCode).To(Equal(http.StatusBadRequest))
			})

			It("should not update the app", func() {
				Expect(lrpBifrost.UpdateCallCount()).To(Equal(0))
			})

			It("should return a response object containing the error", func() {
				verifyResponseObject()
			})

			It("should provide a helpful log message", findLog("app-handler-test.update-app.json-decoding-failed", "myguid"))
		})

		Context("when update fails", func() {
			BeforeEach(func() {
				lrpBifrost.UpdateReturns(errors.New("Failed to update"))
			})

			It("should return a 500 HTTP status code", func() {
				Expect(response.StatusCode).To(Equal(http.StatusInternalServerError))
			})
			It("should provide a helpful log message", findLog("app-handler-test.update-app.bifrost-failed", "myguid"))

			It("shoud return a response object containing the error", func() {
				verifyResponseObject()
			})
		})
	})

	Context("Stop an app", func() {
		var (
			path     string
			response *http.Response
		)

		BeforeEach(func() {
			path = "/apps/app_1234/version_1234/stop"
		})

		JustBeforeEach(func() {
			req, err := http.NewRequest("PUT", ts.URL+path, nil)
			Expect(err).NotTo(HaveOccurred())

			client := &http.Client{}
			response, err = client.Do(req)
			Expect(err).ToNot(HaveOccurred())
		})

		It("should return a 200 HTTP status code", func() {
			Expect(response.StatusCode).To(Equal(http.StatusOK))
		})

		It("should stop the app", func() {
			Expect(lrpBifrost.StopCallCount()).To(Equal(1))
		})

		It("should target the right app", func() {
			_, identifier := lrpBifrost.StopArgsForCall(0)
			Expect(identifier.GUID).To(Equal("app_1234"))
			Expect(identifier.Version).To(Equal("version_1234"))
		})

		Context("when app stop is not successful", func() {
			BeforeEach(func() {
				lrpBifrost.StopReturns(errors.New("someting-bad-happened"))
			})

			It("should return a 500 HTTP status code", func() {
				Expect(response.StatusCode).To(Equal(http.StatusInternalServerError))
			})

			It("should provide a helpful log message", findLog("app-handler-test.stop-app.bifrost-failed", "app_1234"))
		})
	})

	Context("Stop an app instance", func() {
		var (
			path     string
			response *http.Response
		)

		BeforeEach(func() {
			path = "/apps/app_1234/version_1234/stop/1"
		})

		JustBeforeEach(func() {
			req, err := http.NewRequest("PUT", ts.URL+path, nil)
			Expect(err).NotTo(HaveOccurred())

			client := &http.Client{}
			response, err = client.Do(req)
			Expect(err).ToNot(HaveOccurred())
		})

		It("should return a 200 HTTP status code", func() {
			Expect(response.StatusCode).To(Equal(http.StatusOK))
		})

		It("should stop the app instance", func() {
			Expect(lrpBifrost.StopInstanceCallCount()).To(Equal(1))
		})

		It("should target the right app and the right instance index", func() {
			_, identifier, index := lrpBifrost.StopInstanceArgsForCall(0)
			Expect(identifier.GUID).To(Equal("app_1234"))
			Expect(identifier.Version).To(Equal("version_1234"))
			Expect(index).To(Equal(uint(1)))
		})

		Context("when app stop is not successful", func() {
			Context("because the app index does not exist", func() {
				BeforeEach(func() {
					lrpBifrost.StopInstanceReturns(errors.Wrap(eirini.ErrInvalidInstanceIndex, "something-bad-happened"))
				})

				It("should return a 404 HTTP status code", func() {
					Expect(response.StatusCode).To(Equal(http.StatusBadRequest))
				})

				It("should provide a helpful log message", findLog("app-handler-test.stop-app-instance.bifrost-failed", "app_1234"))
			})

			Context("because of some internal error", func() {
				BeforeEach(func() {
					lrpBifrost.StopInstanceReturns(errors.New("something-bad-happened"))
				})

				It("should return a 500 HTTP status code", func() {
					Expect(response.StatusCode).To(Equal(http.StatusInternalServerError))
				})

				It("should provide a helpful log message", findLog("app-handler-test.stop-app-instance.bifrost-failed", "app_1234"))
			})

			Context("because of a invalid index", func() {
				BeforeEach(func() {
					path = "/apps/app_1234/version_1234/stop/x"
				})

				It("should return a 400 HTTP status code", func() {
					Expect(response.StatusCode).To(Equal(http.StatusBadRequest))
				})

				It("should provide a helpful log message", findLog("app-handler-test.stop-app-instance.parsing-instance-index-failed", "app_1234"))
			})

			Context("because of a negative index", func() {
				BeforeEach(func() {
					path = "/apps/app_1234/version_1234/stop/-1"
				})

				It("should return a 400 HTTP status code", func() {
					Expect(response.StatusCode).To(Equal(http.StatusBadRequest))
				})

				It("should provide a helpful log message", findLog("app-handler-test.stop-app-instance.parsing-instance-index-failed", "app_1234"))
			})
		})
	})
})

func createSchedulingInfos() []cf.DesiredLRPSchedulingInfo {
	schedInfo1 := cf.DesiredLRPSchedulingInfo{}
	schedInfo1.ProcessGUID = "1234"

	schedInfo2 := cf.DesiredLRPSchedulingInfo{}
	schedInfo2.ProcessGUID = "5678"

	return []cf.DesiredLRPSchedulingInfo{
		schedInfo1,
		schedInfo2,
	}
}

func readBody(body *bytes.Buffer) ([]byte, error) {
	bytes, err := ioutil.ReadAll(body)
	if err != nil {
		return nil, err
	}

	return bytes, nil
}

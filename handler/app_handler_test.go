package handler_test

import (
	"bytes"
	"encoding/json"
	"errors"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"strings"

	"code.cloudfoundry.org/bbs/models"
	"code.cloudfoundry.org/eirini/eirinifakes"
	. "code.cloudfoundry.org/eirini/handler"
	"code.cloudfoundry.org/eirini/models/cf"
	"code.cloudfoundry.org/lager/lagertest"
	"github.com/gogo/protobuf/jsonpb"
	"github.com/julienschmidt/httprouter"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("AppHandler", func() {
	var (
		bifrost *eirinifakes.FakeBifrost
		stager  *eirinifakes.FakeStager
		lager   *lagertest.TestLogger
	)

	BeforeEach(func() {
		bifrost = new(eirinifakes.FakeBifrost)
		stager = new(eirinifakes.FakeStager)
		lager = lagertest.NewTestLogger("app-handler-test")
	})

	Context("Desire an app", func() {
		var (
			path     string
			body     string
			response *http.Response
		)

		BeforeEach(func() {
			path = "/apps/myguid"
			body = `{
				"process_guid" : "myguid",
				"start_command": "./start",
				"environment": { "env_var": "env_var_value" },
				"instances": 5,
				"memory_mb": 123,
				"cpu_weight": 10,
				"last_updated":"1529073295.9",
				"health_check_type":"http",
				"health_check_http_endpoint":"/healthz",
				"health_check_timeout_ms":400,
				"volume_mounts": [
					{
						"mount_dir": "/var/vcap/data/e1df89b4-33de-4d72-b471-5495222177c8",
						"volume_id":"vol1"
					}
				],
				"ports":[8080,7777]
			}`
		})

		JustBeforeEach(func() {
			ts := httptest.NewServer(New(bifrost, stager, lager))
			req, err := http.NewRequest("PUT", ts.URL+path, bytes.NewReader([]byte(body)))
			Expect(err).NotTo(HaveOccurred())

			client := &http.Client{}
			response, err = client.Do(req)
			Expect(err).ToNot(HaveOccurred())
		})

		It("should call the bifrost with the desired LRPs request from Cloud Controller", func() {
			expectedRequest := cf.DesireLRPRequest{
				ProcessGUID:             "myguid",
				StartCommand:            "./start",
				Environment:             map[string]string{"env_var": "env_var_value"},
				NumInstances:            5,
				MemoryMB:                123,
				CPUWeight:               10,
				LastUpdated:             "1529073295.9",
				HealthCheckType:         "http",
				HealthCheckHTTPEndpoint: "/healthz",
				HealthCheckTimeoutMs:    400,
				Ports:                   []int32{8080, 7777},
				VolumeMounts: []cf.VolumeMount{
					{
						MountDir: "/var/vcap/data/e1df89b4-33de-4d72-b471-5495222177c8",
						VolumeID: "vol1",
					},
				},
			}

			Expect(bifrost.TransferCallCount()).To(Equal(1))
			_, request := bifrost.TransferArgsForCall(0)
			Expect(request).To(Equal(expectedRequest))
		})

		Context("When the endpoint process guid does not match the desired app process guid", func() {

			BeforeEach(func() {
				body = `{"process_guid" : "myguid2", "start_command": "./start", "environment": [ { "name": "env_var", "value": "env_var_value" } ], "num_instances": 5}`
			})

			It("should return BadRequest status", func() {
				Expect(response.StatusCode).To(Equal(http.StatusBadRequest))
			})
		})

	})

	Context("List Apps", func() {
		var (
			appHandler           *App
			responseRecorder     *httptest.ResponseRecorder
			expectedJSONResponse string
			schedInfos           []*models.DesiredLRPSchedulingInfo
		)

		BeforeEach(func() {
			schedInfos = createSchedulingInfos()
		})

		JustBeforeEach(func() {
			req, err := http.NewRequest("", "/apps", nil)
			Expect(err).ToNot(HaveOccurred())
			responseRecorder = httptest.NewRecorder()
			appHandler = NewAppHandler(bifrost, lager)
			bifrost.ListReturns(schedInfos, nil)
			appHandler.List(responseRecorder, req, httprouter.Params{})
			expectedResponse := models.DesiredLRPSchedulingInfosResponse{
				DesiredLrpSchedulingInfos: schedInfos,
			}
			expectedJSONResponse, err = (&jsonpb.Marshaler{Indent: "", OrigName: true}).MarshalToString(&expectedResponse)
			Expect(err).ToNot(HaveOccurred())
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
				schedInfos = []*models.DesiredLRPSchedulingInfo{}
			})

			It("should return an empty list of DesiredLRPSchedulingInfo", func() {
				Expect(responseRecorder.Code).To(Equal(http.StatusOK))

				body, err := readBody(responseRecorder.Body)
				Expect(err).ToNot(HaveOccurred())

				Expect(strings.Trim(string(body), "\n")).To(Equal(expectedJSONResponse))
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
			var responseObj models.DesiredLRPLifecycleResponse
			err := json.NewDecoder(response.Body).Decode(&responseObj)
			Expect(err).ToNot(HaveOccurred())

			Expect(responseObj.Error.Message).ToNot(BeNil())
		}

		BeforeEach(func() {
			path = "/apps/myguid"
			body = `{"guid": "app-id", "version": "version-id", "update": {"instances": 5}}`
		})

		JustBeforeEach(func() {
			ts := httptest.NewServer(New(bifrost, stager, lager))
			req, err := http.NewRequest("POST", ts.URL+path, bytes.NewReader([]byte(body)))
			Expect(err).NotTo(HaveOccurred())

			client := &http.Client{}
			response, err = client.Do(req)
			Expect(err).ToNot(HaveOccurred())
		})

		Context("when the update is successful", func() {
			BeforeEach(func() {
				bifrost.UpdateReturns(nil)
			})

			It("should return a 200 HTTP stauts code", func() {
				Expect(response.StatusCode).To(Equal(http.StatusOK))
			})

			It("should translate the request", func() {
				Expect(bifrost.UpdateCallCount()).To(Equal(1))
				_, request := bifrost.UpdateArgsForCall(0)
				Expect(request.GUID).To(Equal("app-id"))
				Expect(request.Version).To(Equal("version-id"))
				Expect(*request.Update.Instances).To(Equal(int32(5)))
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
				Expect(bifrost.UpdateCallCount()).To(Equal(0))
			})

			It("should return a response object containing the error", func() {
				verifyResponseObject()
			})
		})

		Context("when update fails", func() {
			BeforeEach(func() {
				bifrost.UpdateReturns(errors.New("Failed to update"))
			})

			It("should return a 500 HTTP status code", func() {
				Expect(response.StatusCode).To(Equal(http.StatusInternalServerError))
			})

			It("shoud return a response object containing the error", func() {
				verifyResponseObject()
			})

		})
	})

	Context("Get an app", func() {
		var (
			path       string
			response   *http.Response
			desiredLRP *models.DesiredLRP
		)

		BeforeEach(func() {
			path = "/apps/guid_1234/version_1234"
		})

		JustBeforeEach(func() {
			ts := httptest.NewServer(New(bifrost, stager, lager))
			req, err := http.NewRequest("GET", ts.URL+path, nil)
			Expect(err).NotTo(HaveOccurred())

			client := &http.Client{}
			response, err = client.Do(req)
			Expect(err).ToNot(HaveOccurred())

		})

		It("should use the bifrost to get the app", func() {
			Expect(bifrost.GetAppCallCount()).To(Equal(1))
			_, identifier := bifrost.GetAppArgsForCall(0)
			Expect(identifier.GUID).To(Equal("guid_1234"))
			Expect(identifier.Version).To(Equal("version_1234"))
		})

		Context("when the app exists", func() {
			BeforeEach(func() {
				desiredLRP = &models.DesiredLRP{
					ProcessGuid: "guid_1234-version_1234",
					Instances:   5,
				}
				bifrost.GetAppReturns(desiredLRP)
			})

			It("should return a 200 HTTP status code", func() {
				Expect(response.StatusCode).To(Equal(http.StatusOK))
			})

			It("should return the DesiredLRP in the response body", func() {
				var getLRPResponse models.DesiredLRPResponse
				err := json.NewDecoder(response.Body).Decode(&getLRPResponse)
				Expect(err).ToNot(HaveOccurred())

				actualLRP := getLRPResponse.DesiredLrp
				Expect(actualLRP.ProcessGuid).To(Equal("guid_1234-version_1234"))
				Expect(actualLRP.Instances).To(Equal(int32(5)))
			})

		})

		Context("when the app does not exist", func() {
			BeforeEach(func() {
				bifrost.GetAppReturns(nil)
			})

			It("should return a 404 HTTP status code", func() {
				Expect(response.StatusCode).To(Equal(http.StatusNotFound))
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
			ts := httptest.NewServer(New(bifrost, stager, lager))
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
			Expect(bifrost.StopCallCount()).To(Equal(1))
		})

		It("should target the right app", func() {
			_, identifier := bifrost.StopArgsForCall(0)
			Expect(identifier.GUID).To(Equal("app_1234"))
			Expect(identifier.Version).To(Equal("version_1234"))
		})

		Context("when app stop is not successful", func() {
			BeforeEach(func() {
				bifrost.StopReturns(errors.New("someting-bad-happened"))
			})

			It("should return a 500 HTTP status code", func() {
				Expect(response.StatusCode).To(Equal(http.StatusInternalServerError))
			})

			It("should log the failure", func() {
				messages := lager.LogMessages()
				Expect(messages).To(ConsistOf("app-handler-test.stop-app-failed"))
			})

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
			ts := httptest.NewServer(New(bifrost, stager, lager))
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
			Expect(bifrost.StopInstanceCallCount()).To(Equal(1))
		})

		It("should target the right app and the right instance index", func() {
			_, identifier, index := bifrost.StopInstanceArgsForCall(0)
			Expect(identifier.GUID).To(Equal("app_1234"))
			Expect(identifier.Version).To(Equal("version_1234"))
			Expect(index).To(Equal(uint(1)))
		})

		Context("when app stop is not successful", func() {
			Context("because of some internal error", func() {
				BeforeEach(func() {
					bifrost.StopInstanceReturns(errors.New("something-bad-happened"))
				})

				It("should return a 500 HTTP status code", func() {
					Expect(response.StatusCode).To(Equal(http.StatusInternalServerError))
				})

				It("should log the failure", func() {
					messages := lager.LogMessages()
					Expect(messages).To(ConsistOf("app-handler-test.stop-app-instance-failed"))
				})
			})

			Context("because of a invalid index", func() {
				BeforeEach(func() {
					path = "/apps/app_1234/version_1234/stop/x"
				})

				It("should return a 400 HTTP status code", func() {
					Expect(response.StatusCode).To(Equal(http.StatusBadRequest))
				})

				It("should log the failure", func() {
					messages := lager.LogMessages()
					Expect(messages).To(ConsistOf("app-handler-test.stop-app-instance-failed"))
				})
			})

			Context("because of a negative index", func() {
				BeforeEach(func() {
					path = "/apps/app_1234/version_1234/stop/-1"
				})

				It("should return a 400 HTTP status code", func() {
					Expect(response.StatusCode).To(Equal(http.StatusBadRequest))
				})

				It("should log the failure", func() {
					messages := lager.LogMessages()
					Expect(messages).To(ConsistOf("app-handler-test.stop-app-instance-failed"))
				})
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
			bifrost.GetInstancesReturns(instances, nil)
		})

		JustBeforeEach(func() {
			ts := httptest.NewServer(New(bifrost, stager, lager))
			req, err := http.NewRequest("GET", ts.URL+path, nil)
			Expect(err).NotTo(HaveOccurred())

			client := &http.Client{}
			response, err = client.Do(req)
			Expect(err).ToNot(HaveOccurred())
		})

		It("should use bifrost to get all instances", func() {
			Expect(bifrost.GetInstancesCallCount()).To(Equal(1))
			_, identifier := bifrost.GetInstancesArgsForCall(0)
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
				bifrost.GetInstancesReturns([]*cf.Instance{}, errors.New("not found"))
			})

			It("returns the error in the response", func() {
				expectedResponse := `
					{
						"error": "not found",
						"process_guid": "guid_1234-version_1234",
						"instances": []
					}`
				body, err := ioutil.ReadAll(response.Body)
				Expect(err).ToNot(HaveOccurred())
				Expect(string(body)).To(MatchJSON(expectedResponse))
			})
		})
	})
})

func createSchedulingInfos() []*models.DesiredLRPSchedulingInfo {
	schedInfo1 := &models.DesiredLRPSchedulingInfo{}
	schedInfo1.ProcessGuid = "1234"

	schedInfo2 := &models.DesiredLRPSchedulingInfo{}
	schedInfo2.ProcessGuid = "5678"

	return []*models.DesiredLRPSchedulingInfo{
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

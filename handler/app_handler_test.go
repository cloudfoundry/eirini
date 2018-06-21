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
	"code.cloudfoundry.org/lager"
	"code.cloudfoundry.org/lager/lagertest"
	"github.com/gogo/protobuf/jsonpb"
	"github.com/julienschmidt/httprouter"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("AppHandler", func() {

	var (
		bifrost *eirinifakes.FakeBifrost
		lager   lager.Logger
	)

	BeforeEach(func() {
		bifrost = new(eirinifakes.FakeBifrost)
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
			body = `{"process_guid" : "myguid", "start_command": "./start", "environment": { "env_var": "env_var_value" }, "instances": 5, "last_updated":"1529073295.9"}`
		})

		JustBeforeEach(func() {
			ts := httptest.NewServer(New(bifrost, lager))
			req, err := http.NewRequest("PUT", ts.URL+path, bytes.NewReader([]byte(body)))
			Expect(err).NotTo(HaveOccurred())

			client := &http.Client{}
			response, err = client.Do(req)
			Expect(err).ToNot(HaveOccurred())
		})

		It("should call the bifrost with the desired LRPs request from Cloud Controller", func() {
			expectedRequest := cf.DesireLRPRequest{
				ProcessGuid:  "myguid",
				StartCommand: "./start",
				Environment:  map[string]string{"env_var": "env_var_value"},
				NumInstances: 5,
				LastUpdated:  "1529073295.9",
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
			appHandler           *AppHandler
			responseRecorder     *httptest.ResponseRecorder
			req                  *http.Request
			expectedJsonResponse string
			schedInfos           []*models.DesiredLRPSchedulingInfo
		)

		BeforeEach(func() {
			schedInfos = createSchedulingInfos()
		})

		JustBeforeEach(func() {
			req, _ = http.NewRequest("", "/apps", nil)
			responseRecorder = httptest.NewRecorder()
			appHandler = NewAppHandler(bifrost, lager)
			bifrost.ListReturns(schedInfos, nil)
			appHandler.List(responseRecorder, req, httprouter.Params{})
			expectedResponse := models.DesiredLRPSchedulingInfosResponse{
				DesiredLrpSchedulingInfos: schedInfos,
			}
			expectedJsonResponse, _ = (&jsonpb.Marshaler{Indent: "", OrigName: true}).MarshalToString(&expectedResponse)
		})

		Context("When there are existing apps", func() {
			It("should list all DesiredLRPSchedulingInfos as JSON in the response body", func() {
				Expect(responseRecorder.Code).To(Equal(http.StatusOK))

				body, err := readBody(responseRecorder.Body)
				Expect(err).ToNot(HaveOccurred())

				Expect(strings.Trim(string(body), "\n")).To(Equal(string(expectedJsonResponse)))
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

				Expect(strings.Trim(string(body), "\n")).To(Equal(string(expectedJsonResponse)))
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
			body = `{"process_guid": "myguid", "update": {"instances": 5}}`
		})

		JustBeforeEach(func() {
			ts := httptest.NewServer(New(bifrost, lager))
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
				Expect(request.ProcessGuid).To(Equal("myguid"))
				Expect(*request.Update.Instances).To(Equal(int32(5)))
			})
		})

		Context("when the endpoint guid does not match the one in the body", func() {

			BeforeEach(func() {
				body = `{"process_guid": "anotherGUID", "update": {"instances": 5}}`
			})

			It("should return a 400 HTTP status code", func() {
				Expect(response.StatusCode).To(Equal(http.StatusBadRequest))
			})

			It("should return a response object containing the error", func() {
				verifyResponseObject()
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

	Context("get an app", func() {
		var (
			path       string
			response   *http.Response
			desiredLRP *models.DesiredLRP
		)

		BeforeEach(func() {
			path = "/app/guid_1234"
		})

		JustBeforeEach(func() {
			ts := httptest.NewServer(New(bifrost, lager))
			req, err := http.NewRequest("GET", ts.URL+path, nil)
			Expect(err).NotTo(HaveOccurred())

			client := &http.Client{}
			response, err = client.Do(req)
			Expect(err).ToNot(HaveOccurred())

		})

		It("should use the bifrost to get the app", func() {
			Expect(bifrost.GetCallCount()).To(Equal(1))
			_, guid := bifrost.GetArgsForCall(0)
			Expect(guid).To(Equal("guid_1234"))
		})

		Context("when the app exists", func() {
			BeforeEach(func() {
				desiredLRP = &models.DesiredLRP{
					ProcessGuid: "guid_1234",
					Instances:   5,
				}
				bifrost.GetReturns(desiredLRP)
			})

			It("should return a 200 HTTP status code", func() {
				Expect(response.StatusCode).To(Equal(http.StatusOK))
			})

			It("should return the DesiredLRP in the response body", func() {
				var getLRPResponse models.DesiredLRPResponse
				err := json.NewDecoder(response.Body).Decode(&getLRPResponse)
				Expect(err).ToNot(HaveOccurred())

				actualLRP := getLRPResponse.DesiredLrp
				Expect(actualLRP.ProcessGuid).To(Equal("guid_1234"))
				Expect(actualLRP.Instances).To(Equal(int32(5)))
			})

		})

		Context("when the app does not exist", func() {
			BeforeEach(func() {
				bifrost.GetReturns(nil)
			})

			It("should return a 404 HTTP status code", func() {
				Expect(response.StatusCode).To(Equal(http.StatusNotFound))
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

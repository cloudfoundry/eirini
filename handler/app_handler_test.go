package handler_test

import (
	"bytes"
	"net/http"
	"net/http/httptest"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"code.cloudfoundry.org/bbs/models"
	"code.cloudfoundry.org/eirini/eirinifakes"
	. "code.cloudfoundry.org/eirini/handler"
	"code.cloudfoundry.org/lager/lagertest"
	"code.cloudfoundry.org/runtimeschema/cc_messages"
)

var _ = Describe("AppHandler", func() {

	Context("Desire an app", func() {

		var (
			path     string
			body     string
			bifrost  *eirinifakes.FakeBifrost
			response *http.Response
		)

		BeforeEach(func() {
			path = "/apps/myguid"
			body = `{"process_guid" : "myguid", "start_command": "./start", "environment": [ { "name": "env_var", "value": "env_var_value" } ], "num_instances": 5}`
		})

		JustBeforeEach(func() {
			lager := lagertest.NewTestLogger("app-handler-test")
			bifrost = new(eirinifakes.FakeBifrost)
			ts := httptest.NewServer(New(bifrost, lager))
			req, err := http.NewRequest("PUT", ts.URL+path, bytes.NewReader([]byte(body)))
			Expect(err).NotTo(HaveOccurred())

			client := &http.Client{}
			response, err = client.Do(req)
			Expect(err).ToNot(HaveOccurred())
		})

		It("should return OK status", func() {
			Expect(response.StatusCode).To(Equal(http.StatusOK))
		})

		It("should call the bifrost with the desired LRPs request from Cloud Controller", func() {
			expectedRequest := cc_messages.DesireAppRequestFromCC{
				ProcessGuid:  "myguid",
				StartCommand: "./start",
				Environment:  []*models.EnvironmentVariable{&models.EnvironmentVariable{Name: "env_var", Value: "env_var_value"}},
				NumInstances: 5,
			}

			Expect(bifrost.TransferCallCount()).To(Equal(1))
			_, messages := bifrost.TransferArgsForCall(0)
			Expect(messages[0]).To(Equal(expectedRequest))
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
})

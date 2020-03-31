package handler_test

import (
	"bytes"
	"errors"
	"net/http"
	"net/http/httptest"

	"code.cloudfoundry.org/eirini/eirinifakes"
	. "code.cloudfoundry.org/eirini/handler"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"code.cloudfoundry.org/eirini/opi"
	"code.cloudfoundry.org/lager/lagertest"
)

var _ = Describe("TaskHandler", func() {

	var (
		ts     *httptest.Server
		logger *lagertest.TestLogger

		taskDesirer *eirinifakes.FakeTaskDesirer
		response    *http.Response
		body        string
		path        string
		method      string
	)

	BeforeEach(func() {
		logger = lagertest.NewTestLogger("test")
		taskDesirer = new(eirinifakes.FakeTaskDesirer)

		method = "POST"
		path = "/tasks/guid_1234"
		body = `{
				"app_guid": "our-app-id",
				"environment": [{"name": "HOWARD", "value": "the alien"}],
				"completion_callback": "example.com/call/me/maybe",
				"lifecycle": {
          "buildpack_lifecycle": {
						"droplet_guid": "some-guid",
						"droplet_hash": "some-hash",
					  "start_command": "some command"
					}
				}
			}`
	})

	JustBeforeEach(func() {
		handler := New(nil, nil, nil, taskDesirer, "foo://registry", logger)
		ts = httptest.NewServer(handler)
		req, err := http.NewRequest(method, ts.URL+path, bytes.NewReader([]byte(body)))
		Expect(err).NotTo(HaveOccurred())

		client := &http.Client{}
		response, err = client.Do(req)
		Expect(err).ToNot(HaveOccurred())
	})

	AfterEach(func() {
		ts.Close()
	})

	It("should return 202 Accepted code", func() {
		Expect(response.StatusCode).To(Equal(http.StatusAccepted))
	})

	It("should desire the task", func() {
		Expect(taskDesirer.DesireCallCount()).To(Equal(1))
		Expect(*taskDesirer.DesireArgsForCall(0)).To(Equal(opi.Task{
			TaskGUID: "guid_1234",
			AppGUID:  "our-app-id",
			Env: map[string]string{
				"HOWARD":        "the alien",
				"HOME":          "/home/vcap/app",
				"PATH":          "/usr/local/bin:/usr/bin:/bin",
				"USER":          "vcap",
				"TMPDIR":        "/home/vcap/tmp",
				"START_COMMAND": "some command",
			},
			Image: "foo://registry/cloudfoundry/some-guid:some-hash",
		}))
	})

	Context("when the request body cannot be unmarshalled", func() {
		BeforeEach(func() {
			body = "random stuff"
		})

		It("should return 400 Bad Request code", func() {
			Expect(response.StatusCode).To(Equal(http.StatusBadRequest))
		})

		It("should not desire tasks", func() {
			Expect(taskDesirer.DesireCallCount()).To(Equal(0))
		})
	})

	Context("when the lifecycle is uknown", func() {
		BeforeEach(func() {
			body = `{
				"lifecycle": {
          "amazing_lifecycle": {
					}
				}
			}`
		})

		It("should return 400 Bad Request code", func() {
			Expect(response.StatusCode).To(Equal(http.StatusBadRequest))
		})

		It("should not desire tasks", func() {
			Expect(taskDesirer.DesireCallCount()).To(Equal(0))
		})
	})

	Context("when the task desirer fails", func() {
		BeforeEach(func() {
			taskDesirer.DesireReturns(errors.New("undesired-error"))
		})

		It("should return 500 Internal Server Error code", func() {
			Expect(response.StatusCode).To(Equal(http.StatusInternalServerError))
		})
	})
})

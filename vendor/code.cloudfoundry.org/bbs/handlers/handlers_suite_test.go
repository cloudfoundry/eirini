package handlers_test

import (
	"bytes"
	"io"
	"net/http"
	"strings"

	"code.cloudfoundry.org/bbs/serviceclient/serviceclientfakes"
	"code.cloudfoundry.org/rep/repfakes"
	"github.com/gogo/protobuf/proto"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"testing"
)

func TestHandlers(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Handlers Suite")
}

var (
	fakeServiceClient    *serviceclientfakes.FakeServiceClient
	fakeRepClient        *repfakes.FakeClient
	fakeRepClientFactory *repfakes.FakeClientFactory
)

var _ = BeforeEach(func() {
	fakeServiceClient = new(serviceclientfakes.FakeServiceClient)
	fakeRepClientFactory = new(repfakes.FakeClientFactory)
	fakeRepClient = new(repfakes.FakeClient)
	fakeRepClientFactory.CreateClientReturns(fakeRepClient, nil)
})

func newTestRequest(body interface{}) *http.Request {
	var reader io.Reader
	switch body := body.(type) {
	case io.Reader:
		reader = body
	case string:
		reader = strings.NewReader(body)
	case []byte:
		reader = bytes.NewReader(body)
	case proto.Message:
		protoBytes, err := proto.Marshal(body)
		Expect(err).NotTo(HaveOccurred())
		reader = bytes.NewReader(protoBytes)
	default:
		panic("cannot create test request")
	}

	request, err := http.NewRequest("", "", reader)
	Expect(err).NotTo(HaveOccurred())
	return request
}

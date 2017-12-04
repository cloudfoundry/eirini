package dropsonde_test

import (
	"net/http"
	"reflect"

	"github.com/cloudfoundry/dropsonde"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Autowire", func() {

	Describe("Initialize", func() {
		It("resets the HTTP default transport to be instrumented", func() {
			dropsonde.InitializeWithEmitter(&dropsonde.NullEventEmitter{})
			Expect(reflect.TypeOf(http.DefaultTransport).Elem().Name()).To(Equal("instrumentedCancelableRoundTripper"))
		})
	})

	Describe("CreateDefaultEmitter", func() {
		Context("with origin missing", func() {
			It("returns a NullEventEmitter", func() {
				err := dropsonde.Initialize("localhost:2343", "")
				Expect(err).To(HaveOccurred())

				emitter := dropsonde.AutowiredEmitter()
				Expect(emitter).ToNot(BeNil())
				Expect(emitter).To(Equal(dropsonde.DefaultEmitter))
				nullEmitter := &dropsonde.NullEventEmitter{}
				Expect(emitter).To(BeAssignableToTypeOf(nullEmitter))
			})
		})
	})
})

type FakeHandler struct{}

func (fh FakeHandler) ServeHTTP(rw http.ResponseWriter, r *http.Request) {}

type FakeRoundTripper struct{}

func (frt FakeRoundTripper) RoundTrip(req *http.Request) (*http.Response, error) {
	return nil, nil
}

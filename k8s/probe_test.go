package k8s_test

import (
	. "code.cloudfoundry.org/eirini/k8s"
	"code.cloudfoundry.org/eirini/opi"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
	_ "k8s.io/client-go/plugin/pkg/client/auth/oidc"
)

var _ = Describe("LivenessProbeCreator", func() {

	var (
		probe *v1.Probe
		lrp   *opi.LRP
	)

	BeforeEach(func() {
		lrp = &opi.LRP{
			Health: opi.Healtcheck{
				Endpoint:  "/healthz",
				Port:      8080,
				TimeoutMs: 3000,
			},
		}
	})

	JustBeforeEach(func() {
		probe = CreateLivenessProbe(lrp)
	})

	Context("When healthcheck type is HTTP", func() {

		BeforeEach(func() {
			lrp.Health.Type = "http"
		})

		It("creates a probe with HTTPGet action", func() {
			Expect(probe).To(Equal(&v1.Probe{
				Handler: v1.Handler{
					HTTPGet: &v1.HTTPGetAction{
						Path: "/healthz",
						Port: intstr.IntOrString{Type: intstr.Int, IntVal: 8080},
					},
				},
				TimeoutSeconds: 3,
			}))
		})

	})

	Context("When healthcheck type is PORT", func() {

		BeforeEach(func() {
			lrp.Health.Type = "port"
		})

		It("creates a probe with HTTPGet action", func() {
			Expect(probe).To(Equal(&v1.Probe{
				Handler: v1.Handler{
					TCPSocket: &v1.TCPSocketAction{
						Port: intstr.IntOrString{Type: intstr.Int, IntVal: 8080},
					},
				},
				TimeoutSeconds: 3,
			}))
		})
	})

	Context("When timeout is not a whole number", func() {

		BeforeEach(func() {
			lrp.Health.Type = "http"
			lrp.Health.TimeoutMs = 5700
		})

		It("rounds it down", func() {
			Expect(probe.TimeoutSeconds).To(Equal(int32(5)))
		})

	})

	Context("When healthcheck information is missing", func() {

		BeforeEach(func() {
			lrp = &opi.LRP{}
		})

		It("returns nil", func() {
			Expect(probe).To(BeNil())
		})

	})
})

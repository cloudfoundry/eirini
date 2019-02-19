package k8s_test

import (
	. "code.cloudfoundry.org/eirini/k8s"
	"code.cloudfoundry.org/eirini/opi"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
	_ "k8s.io/client-go/plugin/pkg/client/auth/oidc"
)

var _ = Describe("PrrobeCreator", func() {

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

	Context("LivenessProbeCreator", func() {

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
					InitialDelaySeconds: 3,
					FailureThreshold:    4,
				}))
			})

		})

		Context("When healthcheck type is Port", func() {

			BeforeEach(func() {
				lrp.Health.Type = "port"
			})

			It("creates a probe with TCPSocket action", func() {
				Expect(probe).To(Equal(&v1.Probe{
					Handler: v1.Handler{
						TCPSocket: &v1.TCPSocketAction{
							Port: intstr.IntOrString{Type: intstr.Int, IntVal: 8080},
						},
					},
					InitialDelaySeconds: 3,
					FailureThreshold:    4,
				}))
			})
		})

		Context("When timeout is not a whole number", func() {

			BeforeEach(func() {
				lrp.Health.Type = "http"
				lrp.Health.TimeoutMs = 5700
			})

			It("rounds it down", func() {
				Expect(probe.InitialDelaySeconds).To(Equal(int32(5)))
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

	Context("ReadinessProbeCreator", func() {

		JustBeforeEach(func() {
			probe = CreateReadinessProbe(lrp)
		})

		Context("When Healtcheck type is HTTP", func() {

			BeforeEach(func() {
				lrp.Health.Type = "http"
			})

			It("should create a probe with a HTTP GET action", func() {
				Expect(probe).To(Equal(&v1.Probe{
					Handler: v1.Handler{
						HTTPGet: &v1.HTTPGetAction{
							Path: "/healthz",
							Port: intstr.IntOrString{Type: intstr.Int, IntVal: 8080},
						},
					},
					InitialDelaySeconds: 0,
					FailureThreshold:    1,
				}))
			})
		})

		Context("When Healthcheck type is Port", func() {

			BeforeEach(func() {
				lrp.Health.Type = "port"
			})

			It("should create a probe with a TCPSocket action", func() {
				Expect(probe).To(Equal(&v1.Probe{
					Handler: v1.Handler{
						TCPSocket: &v1.TCPSocketAction{
							Port: intstr.IntOrString{Type: intstr.Int, IntVal: 8080},
						},
					},
					InitialDelaySeconds: 0,
					FailureThreshold:    1,
				}))
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
})

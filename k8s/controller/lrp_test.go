package controller_test

import (
	"encoding/json"
	"net/http"

	"code.cloudfoundry.org/eirini/k8s/controller"
	"code.cloudfoundry.org/eirini/models/cf"
	eiriniv1 "code.cloudfoundry.org/eirini/pkg/apis/lrp/v1"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/ghttp"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

var _ = Describe("Lrp", func() {
	var (
		server        *ghttp.Server
		lrpController *controller.RestLrp
		lrp           eiriniv1.LRP
	)

	BeforeEach(func() {
		server = ghttp.NewServer()

		lrpController = controller.NewRestLrp(
			&http.Client{},
			server.URL(),
		)

		lrp = eiriniv1.LRP{
			ObjectMeta: v1.ObjectMeta{
				Namespace: "some-namespace",
			},
			Spec: eiriniv1.LRPSpec{
				GUID:        "guid",
				Version:     "version",
				ProcessGUID: "process-guid",
				Environment: map[string]string{
					"FOO": "BAR",
					"BAZ": "BAN",
				},
				VolumeMounts: []eiriniv1.VolumeMount{
					{VolumeID: "id", MountDir: "/notroot"},
				},
				Lifecycle: eiriniv1.Lifecycle{
					DockerLifecycle: &eiriniv1.DockerLifecycle{
						Image:   "ubuntu",
						Command: []string{"sleep", "12"},
					},
				},
			},
		}
	})

	Describe("create", func() {
		BeforeEach(func() {
			server.RouteToHandler("PUT", "/apps/process-guid",
				ghttp.VerifyJSONRepresenting(cf.DesireLRPRequest{
					GUID:        "guid",
					Version:     "version",
					ProcessGUID: "process-guid",
					Namespace:   "some-namespace",
					Environment: map[string]string{
						"FOO": "BAR",
						"BAZ": "BAN",
					},
					VolumeMounts: []cf.VolumeMount{
						{VolumeID: "id", MountDir: "/notroot"},
					},
					Lifecycle: cf.Lifecycle{
						DockerLifecycle: &cf.DockerLifecycle{
							Image:   "ubuntu",
							Command: []string{"sleep", "12"},
						},
					},
				}),
			)
		})

		It("sends create request", func() {
			Expect(lrpController.Create(lrp)).To(Succeed())
			Expect(server.ReceivedRequests()).To(HaveLen(1))
		})

		When("the http server retruns an error", func() {
			BeforeEach(func() {
				server.RouteToHandler("PUT", "/apps/process-guid", ghttp.RespondWith(500, nil))
			})

			It("returns the error", func() {
				Expect(lrpController.Create(lrp)).To(MatchError(ContainSubstring("failed to create lrp")))
			})
		})
	})

	Describe("update", func() {

		var oldLRP eiriniv1.LRP

		BeforeEach(func() {
			lrp.Spec.NumInstances = 10
			lrp.Spec.Routes = map[string]json.RawMessage{"foobar": []byte(`{"foo":"bar"}`)}
			lrp.Spec.LastUpdated = "today"

			oldLRP = eiriniv1.LRP{Spec: eiriniv1.LRPSpec{LastUpdated: "yesterday"}}
			server.RouteToHandler("POST", "/apps/process-guid",
				ghttp.VerifyJSONRepresenting(cf.UpdateDesiredLRPRequest{
					GUID:    "guid",
					Version: "version",
					Update: cf.DesiredLRPUpdate{
						Instances:  10,
						Routes:     map[string]json.RawMessage{"foobar": []byte(`{"foo":"bar"}`)},
						Annotation: "today",
					},
				}),
			)
		})

		It("sends update request", func() {
			Expect(lrpController.Update(oldLRP, lrp)).To(Succeed())
			Expect(server.ReceivedRequests()).To(HaveLen(1))
		})

		When("the lastUpdated timestamp has not changed", func() {
			BeforeEach(func() {
				oldLRP.Spec.LastUpdated = "today"
			})

			It("does not send update request", func() {
				Expect(lrpController.Update(oldLRP, lrp)).To(Succeed())
				Expect(server.ReceivedRequests()).To(HaveLen(0))
			})
		})

		When("the http server returns an error", func() {
			BeforeEach(func() {
				server.RouteToHandler("POST", "/apps/process-guid", ghttp.RespondWith(500, nil))
			})

			It("returns the error", func() {
				Expect(lrpController.Update(oldLRP, lrp)).To(MatchError(ContainSubstring("failed to update lrp")))
			})
		})
	})

	Describe("delete", func() {
		BeforeEach(func() {
			server.RouteToHandler("PUT", "/apps/guid/version/stop", ghttp.RespondWith(200, nil))
		})

		It("sends delete request", func() {
			Expect(lrpController.Delete(lrp)).To(Succeed())
			Expect(server.ReceivedRequests()).To(HaveLen(1))
		})

		When("the http server retruns an error", func() {
			BeforeEach(func() {
				server.RouteToHandler("PUT", "/apps/guid/version/stop", ghttp.RespondWith(500, nil))
			})

			It("returns the error", func() {
				Expect(lrpController.Delete(lrp)).To(MatchError(ContainSubstring("failed to delete lrp")))
			})
		})
	})
})

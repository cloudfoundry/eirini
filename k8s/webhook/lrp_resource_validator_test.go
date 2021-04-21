package webhook_test

import (
	"context"
	"encoding/json"
	"net/http"
	"reflect"

	"code.cloudfoundry.org/eirini/k8s/webhook"
	eiriniv1 "code.cloudfoundry.org/eirini/pkg/apis/eirini/v1"
	"code.cloudfoundry.org/lager"
	"code.cloudfoundry.org/lager/lagertest"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	admissionv1 "k8s.io/api/admission/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes/scheme"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"
)

var lrpMutableFields = []string{
	"Image",
	"AppRoutes",
	"Instances",
}

var _ = Describe("LRPResourceValidator", func() {
	var (
		validator    *webhook.LRPResourceValidator
		logger       lager.Logger
		originalLRP  *eiriniv1.LRP
		updatedLRP   *eiriniv1.LRP
		convertToRaw func(interface{}) runtime.RawExtension
		req          admission.Request
		resp         admission.Response
	)

	BeforeEach(func() {
		logger = lagertest.NewTestLogger("instance-index-injector")
		decoder, err := admission.NewDecoder(scheme.Scheme)
		Expect(err).NotTo(HaveOccurred())

		validator = webhook.NewLRPResourceValidator(logger, decoder)
		convertToRaw = toRawExt

		createLRP := func() *eiriniv1.LRP {
			return &eiriniv1.LRP{
				ObjectMeta: metav1.ObjectMeta{
					Name: "the-app",
				},
				Spec: eiriniv1.LRPSpec{
					GUID:        "guid-1234",
					Version:     "version-1234",
					ProcessType: "web",
					AppName:     "app-name",
					AppGUID:     "app-guid-1234",
					OrgName:     "the-org",
					OrgGUID:     "org-guid-1234",
					SpaceName:   "the-space",
					SpaceGUID:   "space-guid-1234",
					Image:       "app/image:latest",
					Command: []string{
						"/bin/bash",
						"-c",
						"echo hello",
					},
					Sidecars: []eiriniv1.Sidecar{
						{
							Name: "the-sidecar",
							Command: []string{
								"do", "nothing",
							},
							MemoryMB: 128,
							Env: map[string]string{
								"FOO": "BAZ",
							},
						},
					},
					PrivateRegistry: &eiriniv1.PrivateRegistry{
						Username: "the-user",
						Password: "the-password",
					},
					Env: map[string]string{
						"FOO": "BAR",
					},
					Health: eiriniv1.Healthcheck{
						Type:      "http",
						Port:      8080,
						Endpoint:  "/end",
						TimeoutMs: 50,
					},
					Ports: []int32{
						8080,
						9090,
					},
					Instances: 1,
					MemoryMB:  1024,
					DiskMB:    512,
					CPUWeight: 3,
					VolumeMounts: []eiriniv1.VolumeMount{
						{
							MountPath: "/a/path/",
							ClaimName: "the-claim",
						},
					},
					LastUpdated: "now",
					UserDefinedAnnotations: map[string]string{
						"user-annotations.io": "aaa",
					},
				},
			}
		}

		originalLRP = createLRP()
		updatedLRP = createLRP()
	})

	JustBeforeEach(func() {
		req = admission.Request{
			AdmissionRequest: admissionv1.AdmissionRequest{
				Operation: admissionv1.Update,
				Object:    convertToRaw(originalLRP),
				OldObject: convertToRaw(updatedLRP),
			},
		}

		resp = validator.Handle(context.Background(), req)
	})

	When("nothing is updated", func() {
		It("allows the change", func() {
			expectAllowResponse(resp)
		})
	})

	When("the intstance count is updated", func() {
		BeforeEach(func() {
			updatedLRP.Spec.Instances = 2
		})

		It("allows the change", func() {
			expectAllowResponse(resp)
		})
	})

	When("the image is updated", func() {
		BeforeEach(func() {
			updatedLRP.Spec.Image = "some-other-image"
		})

		It("allows the change", func() {
			expectAllowResponse(resp)
		})
	})

	When("an immutable field is updated", func() {
		BeforeEach(func() {
			// an empty spec means we are updating every field
			updatedLRP.Spec = eiriniv1.LRPSpec{}
		})

		It("rejects the change", func() {
			Expect(resp.Allowed).To(BeFalse())
			Expect(resp.Result).ToNot(BeNil())
			Expect(resp.Patches).To(BeEmpty())
			Expect(resp.Result.Status).To(Equal("Failure"))
			Expect(resp.Result.Reason).To(Equal(metav1.StatusReasonBadRequest))
			Expect(resp.Result.Code).To(BeNumerically("==", http.StatusBadRequest))

			for _, immutableField := range getLRPImmutableFields() {
				Expect(resp.Result.Message).To(ContainSubstring(immutableField))
			}
		})
	})

	When("an immutable struct field has more than one field updated", func() {
		BeforeEach(func() {
			updatedLRP.Spec.VolumeMounts = []eiriniv1.VolumeMount{
				{
					MountPath: "new-path",
					ClaimName: "new-name",
				},
			}
		})

		It("the immutable field is reported exactly once in the rejection message", func() {
			Expect(resp.Allowed).To(BeFalse())
			Expect(resp.Result.Message).To(Equal("Changing immutable fields not allowed: VolumeMounts"))
		})
	})

	When("the raw objects are not valid LRPs", func() {
		BeforeEach(func() {
			convertToRaw = func(obj interface{}) runtime.RawExtension {
				return runtime.RawExtension{
					Raw: []byte("gibberish"),
				}
			}
		})

		It("errors", func() {
			Expect(resp.Allowed).To(BeFalse())
			Expect(resp.Result.Message).To(ContainSubstring("Error decoding object"))
		})
	})
})

func expectAllowResponse(resp admission.Response) {
	ExpectWithOffset(1, resp.Result).To(BeNil())
	ExpectWithOffset(1, resp.Allowed).To(BeTrue())
	ExpectWithOffset(1, resp.Patches).To(BeEmpty())
}

func getLRPImmutableFields() []string {
	fieldNames := []string{}

	val := reflect.Indirect(reflect.ValueOf(eiriniv1.LRPSpec{}))
	for i := 0; i < val.Type().NumField(); i++ {
		fieldName := val.Type().Field(i).Name
		if isImmutable(fieldName) {
			fieldNames = append(fieldNames, fieldName)
		}
	}

	return fieldNames
}

func isImmutable(field string) bool {
	for _, mutableField := range lrpMutableFields {
		if field == mutableField {
			return false
		}
	}

	return true
}

func toRawExt(obj interface{}) runtime.RawExtension {
	rawObj, err := json.Marshal(obj)
	Expect(err).NotTo(HaveOccurred())

	return runtime.RawExtension{Raw: rawObj}
}

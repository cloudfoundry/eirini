package stset_test

import (
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"math/big"

	"code.cloudfoundry.org/eirini/k8s/shared/sharedfakes"
	"code.cloudfoundry.org/eirini/k8s/stset"
	"code.cloudfoundry.org/eirini/k8s/stset/stsetfakes"
	"code.cloudfoundry.org/eirini/opi"
	"code.cloudfoundry.org/lager"
	"code.cloudfoundry.org/lager/lagertest"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/pkg/errors"
	v1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

var _ = Describe("Desirer", func() {
	var (
		logger                     lager.Logger
		secrets                    *stsetfakes.FakeSecretsCreator
		statefulSets               *stsetfakes.FakeStatefulSetCreator
		lrpToStatefulSetConverter  *stsetfakes.FakeLRPToStatefulSetConverter
		podDisruptionBudgetUpdater *stsetfakes.FakePodDisruptionBudgetUpdater
		desireOptOne, desireOptTwo *sharedfakes.FakeOption

		lrp       *opi.LRP
		desireErr error

		desirer stset.Desirer
	)

	BeforeEach(func() {
		logger = lagertest.NewTestLogger("statefulset-desirer")
		secrets = new(stsetfakes.FakeSecretsCreator)
		statefulSets = new(stsetfakes.FakeStatefulSetCreator)
		lrpToStatefulSetConverter = new(stsetfakes.FakeLRPToStatefulSetConverter)
		lrpToStatefulSetConverter.ConvertStub = func(statefulSetName string, lrp *opi.LRP) (*v1.StatefulSet, error) {
			return &v1.StatefulSet{
				ObjectMeta: metav1.ObjectMeta{
					Name: statefulSetName,
				},
			}, nil
		}

		podDisruptionBudgetUpdater = new(stsetfakes.FakePodDisruptionBudgetUpdater)
		lrp = createLRP("Baldur", []opi.Route{{Hostname: "my.example.route", Port: 1000}})
		desireOptOne = new(sharedfakes.FakeOption)
		desireOptTwo = new(sharedfakes.FakeOption)
		desirer = stset.NewDesirer(logger, secrets, statefulSets, lrpToStatefulSetConverter, podDisruptionBudgetUpdater)
	})

	JustBeforeEach(func() {
		desireErr = desirer.Desire(ctx, "the-namespace", lrp, desireOptOne.Spy, desireOptTwo.Spy)
	})

	It("should succeed", func() {
		Expect(desireErr).NotTo(HaveOccurred())
	})

	It("should set name for the stateful set", func() {
		_, _, statefulSet := statefulSets.CreateArgsForCall(0)
		Expect(statefulSet.Name).To(Equal("baldur-space-foo-34f869d015"))
	})

	It("should call the statefulset client", func() {
		Expect(statefulSets.CreateCallCount()).To(Equal(1))
	})

	It("updates the pod disruption budget", func() {
		Expect(podDisruptionBudgetUpdater.UpdateCallCount()).To(Equal(1))
		_, actualNamespace, actualName, actualLRP := podDisruptionBudgetUpdater.UpdateArgsForCall(0)
		Expect(actualNamespace).To(Equal("the-namespace"))
		Expect(actualName).To(Equal("baldur-space-foo-34f869d015"))
		Expect(actualLRP).To(Equal(lrp))
	})

	When("updating the pod disruption budget fails", func() {
		BeforeEach(func() {
			podDisruptionBudgetUpdater.UpdateReturns(errors.New("update-error"))
		})

		It("returns an error", func() {
			Expect(desireErr).To(MatchError(ContainSubstring("update-error")))
		})
	})

	It("should invoke the opts with the StatefulSet", func() {
		Expect(desireOptOne.CallCount()).To(Equal(1))
		Expect(desireOptTwo.CallCount()).To(Equal(1))

		_, _, statefulSet := statefulSets.CreateArgsForCall(0)
		Expect(desireOptOne.ArgsForCall(0)).To(Equal(statefulSet))
		Expect(desireOptTwo.ArgsForCall(0)).To(Equal(statefulSet))
	})

	It("should set namespace for the stateful set", func() {
		_, namespace, _ := statefulSets.CreateArgsForCall(0)
		Expect(namespace).To(Equal("the-namespace"))
	})

	When("the app name contains unsupported characters", func() {
		BeforeEach(func() {
			lrp = createLRP("Балдър", []opi.Route{{Hostname: "my.example.route", Port: 10000}})
		})

		It("should use the guid as a name", func() {
			_, _, statefulSet := statefulSets.CreateArgsForCall(0)
			Expect(statefulSet.Name).To(Equal("guid_1234-34f869d015"))
		})
	})

	When("the statefulset already exists", func() {
		BeforeEach(func() {
			statefulSets.CreateReturns(nil, k8serrors.NewAlreadyExists(schema.GroupResource{}, "potato"))
		})

		It("does not fail", func() {
			Expect(desireErr).NotTo(HaveOccurred())
		})
	})

	When("creating the statefulset fails", func() {
		BeforeEach(func() {
			statefulSets.CreateReturns(nil, errors.New("potato"))
		})

		It("propagates the error", func() {
			Expect(desireErr).To(MatchError(ContainSubstring("potato")))
		})
	})

	When("the app references a private docker image", func() {
		BeforeEach(func() {
			lrp.PrivateRegistry = &opi.PrivateRegistry{
				Server:   "host",
				Username: "user",
				Password: "password",
			}
		})

		It("should create a private repo secret containing the private repo credentials", func() {
			Expect(secrets.CreateCallCount()).To(Equal(1))
			_, secretNamespace, actualSecret := secrets.CreateArgsForCall(0)
			Expect(secretNamespace).To(Equal("the-namespace"))
			Expect(actualSecret.Name).To(Equal("baldur-space-foo-34f869d015-registry-credentials"))
			Expect(actualSecret.Type).To(Equal(corev1.SecretTypeDockerConfigJson))
			Expect(actualSecret.StringData).To(
				HaveKeyWithValue(
					".dockerconfigjson",
					fmt.Sprintf(
						`{"auths":{"host":{"username":"user","password":"password","auth":"%s"}}}`,
						base64.StdEncoding.EncodeToString([]byte("user:password")),
					),
				),
			)
		})
	})
})

const letterBytes = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ"

func randStringBytes() string {
	b := make([]byte, 10)
	for i := range b {
		randomNumber, err := rand.Int(rand.Reader, big.NewInt(int64(len(letterBytes))))
		Expect(err).NotTo(HaveOccurred())

		b[i] = letterBytes[randomNumber.Int64()]
	}

	return string(b)
}

func createLRP(name string, routes []opi.Route) *opi.LRP {
	lastUpdated := randStringBytes()

	return &opi.LRP{
		LRPIdentifier: opi.LRPIdentifier{
			GUID:    "guid_1234",
			Version: "version_1234",
		},
		ProcessType:     "worker",
		AppName:         name,
		AppGUID:         "premium_app_guid_1234",
		SpaceName:       "space-foo",
		SpaceGUID:       "space-guid",
		TargetInstances: 1,
		OrgName:         "org-foo",
		OrgGUID:         "org-guid",
		Command: []string{
			"/bin/sh",
			"-c",
			"while true; do echo hello; sleep 10;done",
		},
		RunningInstances: 0,
		MemoryMB:         1024,
		DiskMB:           2048,
		CPUWeight:        2,
		Image:            "busybox",
		Ports:            []int32{8888, 9999},
		LastUpdated:      lastUpdated,
		AppURIs:          routes,
		VolumeMounts: []opi.VolumeMount{
			{
				ClaimName: "some-claim",
				MountPath: "/some/path",
			},
		},
		LRP: "original request",
		UserDefinedAnnotations: map[string]string{
			"prometheus.io/scrape": "secret-value",
		},
	}
}

func expectedValFrom(fieldPath string) *corev1.EnvVarSource {
	return &corev1.EnvVarSource{
		FieldRef: &corev1.ObjectFieldSelector{
			APIVersion: "",
			FieldPath:  fieldPath,
		},
	}
}

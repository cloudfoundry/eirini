package stset_test

import (
	"context"
	"encoding/base64"
	"fmt"

	"code.cloudfoundry.org/eirini/api"
	"code.cloudfoundry.org/eirini/k8s/shared/sharedfakes"
	"code.cloudfoundry.org/eirini/k8s/stset"
	"code.cloudfoundry.org/eirini/k8s/stset/stsetfakes"
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
		secrets                    *stsetfakes.FakeSecretsClient
		statefulSets               *stsetfakes.FakeStatefulSetCreator
		lrpToStatefulSetConverter  *stsetfakes.FakeLRPToStatefulSetConverter
		podDisruptionBudgetUpdater *stsetfakes.FakePodDisruptionBudgetUpdater
		desireOptOne, desireOptTwo *sharedfakes.FakeOption

		lrp       *api.LRP
		desireErr error

		desirer stset.Desirer
	)

	BeforeEach(func() {
		logger = lagertest.NewTestLogger("statefulset-desirer")
		secrets = new(stsetfakes.FakeSecretsClient)
		statefulSets = new(stsetfakes.FakeStatefulSetCreator)
		lrpToStatefulSetConverter = new(stsetfakes.FakeLRPToStatefulSetConverter)
		lrpToStatefulSetConverter.ConvertStub = func(statefulSetName string, lrp *api.LRP, _ *corev1.Secret) (*v1.StatefulSet, error) {
			return &v1.StatefulSet{
				ObjectMeta: metav1.ObjectMeta{
					Name: statefulSetName,
				},
			}, nil
		}
		statefulSets.CreateStub = func(_ context.Context, _ string, stSet *v1.StatefulSet) (*v1.StatefulSet, error) {
			return stSet, nil
		}

		podDisruptionBudgetUpdater = new(stsetfakes.FakePodDisruptionBudgetUpdater)
		lrp = createLRP("Baldur")
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
		_, actualStatefulSet, actualLRP := podDisruptionBudgetUpdater.UpdateArgsForCall(0)
		Expect(actualStatefulSet.Namespace).To(Equal("the-namespace"))
		Expect(actualStatefulSet.Name).To(Equal("baldur-space-foo-34f869d015"))
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
			lrp = createLRP("Балдър")
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
		var registrySecret *corev1.Secret

		BeforeEach(func() {
			registrySecret = &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "baldur-secret",
					Namespace: "the-namespace",
				},
			}
			secrets.CreateReturns(registrySecret, nil)

			lrp.PrivateRegistry = &api.PrivateRegistry{
				Server:   "host",
				Username: "user",
				Password: "password",
			}
		})

		It("should create a private repo secret containing the private repo credentials", func() {
			Expect(secrets.CreateCallCount()).To(Equal(1))
			_, secretNamespace, actualSecret := secrets.CreateArgsForCall(0)
			Expect(secretNamespace).To(Equal("the-namespace"))
			Expect(actualSecret.GenerateName).To(Equal("private-registry-"))
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

		It("uses that secret when converting to statefulset", func() {
			Expect(lrpToStatefulSetConverter.ConvertCallCount()).To(Equal(1))
			_, _, actualRegistrySecret := lrpToStatefulSetConverter.ConvertArgsForCall(0)
			Expect(actualRegistrySecret).To(Equal(registrySecret))
		})

		It("sets the statefulset as the secret owner", func() {
			Expect(secrets.SetOwnerCallCount()).To(Equal(1))
			_, actualSecret, actualStatefulSet := secrets.SetOwnerArgsForCall(0)
			Expect(actualSecret.Name).To(Equal("baldur-secret"))
			Expect(actualStatefulSet.GetName()).To(Equal("baldur-space-foo-34f869d015"))
		})

		When("creating the statefulset fails", func() {
			BeforeEach(func() {
				statefulSets.CreateReturns(nil, errors.New("potato"))
			})

			It("deletes the secret", func() {
				Expect(secrets.DeleteCallCount()).To(Equal(1))
				_, actualNamespace, actualName := secrets.DeleteArgsForCall(0)
				Expect(actualNamespace).To(Equal("the-namespace"))
				Expect(actualName).To(Equal("baldur-secret"))
			})

			When("deleting the secret fails", func() {
				BeforeEach(func() {
					secrets.DeleteReturns(errors.New("delete-secret-failed"))
				})

				It("returns a statefulset creation error and a note that the secret is not cleaned up", func() {
					Expect(desireErr).To(MatchError(And(ContainSubstring("potato"), ContainSubstring("delete-secret-failed"))))
				})
			})
		})

		When("setting the statefulset as a secret owner fails", func() {
			BeforeEach(func() {
				secrets.SetOwnerReturns(nil, errors.New("set-owner-failed"))
			})

			It("returns an error", func() {
				Expect(desireErr).To(MatchError(ContainSubstring("set-owner-failed")))
			})
		})
	})
})

const letterBytes = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ"

func expectedValFrom(fieldPath string) *corev1.EnvVarSource {
	return &corev1.EnvVarSource{
		FieldRef: &corev1.ObjectFieldSelector{
			APIVersion: "",
			FieldPath:  fieldPath,
		},
	}
}

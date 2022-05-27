package stset_test

import (
	"fmt"

	"code.cloudfoundry.org/eirini"
	"code.cloudfoundry.org/eirini/api"
	"code.cloudfoundry.org/eirini/k8s/stset"
	"code.cloudfoundry.org/eirini/k8s/stset/stsetfakes"
	"code.cloudfoundry.org/eirini/tests"
	"code.cloudfoundry.org/lager"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/pkg/errors"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const (
	namespace = "testing"
)

var _ = Describe("Get StatefulSet", func() {
	var (
		logger                    lager.Logger
		podGetter                 *stsetfakes.FakePodGetter
		eventGetter               *stsetfakes.FakeEventGetter
		statefulSetGetter         *stsetfakes.FakeStatefulSetByLRPIdentifierGetter
		statefulsetToLRPConverter *stsetfakes.FakeStatefulSetToLRPConverter

		getter stset.Getter
	)

	BeforeEach(func() {
		logger = tests.NewTestLogger("handler-test")
		podGetter = new(stsetfakes.FakePodGetter)
		eventGetter = new(stsetfakes.FakeEventGetter)
		statefulSetGetter = new(stsetfakes.FakeStatefulSetByLRPIdentifierGetter)
		statefulsetToLRPConverter = new(stsetfakes.FakeStatefulSetToLRPConverter)
		statefulsetToLRPConverter.ConvertReturns(&api.LRP{AppName: "baldur-app"}, nil)

		getter = stset.NewGetter(logger, statefulSetGetter, podGetter, eventGetter, statefulsetToLRPConverter)
	})

	Describe("Get", func() {
		It("should use mapper to get LRP", func() {
			st := appsv1.StatefulSet{
				ObjectMeta: metav1.ObjectMeta{
					Name: "baldur",
				},
			}

			statefulSetGetter.GetByLRPIdentifierReturns([]appsv1.StatefulSet{st}, nil)
			lrp, _ := getter.Get(ctx, api.LRPIdentifier{GUID: "guid_1234", Version: "version_1234"})
			Expect(statefulsetToLRPConverter.ConvertCallCount()).To(Equal(1))
			Expect(lrp.AppName).To(Equal("baldur-app"))
		})

		When("the app does not exist", func() {
			BeforeEach(func() {
				statefulSetGetter.GetByLRPIdentifierReturns([]appsv1.StatefulSet{}, nil)
			})

			It("should return an error", func() {
				_, err := getter.Get(ctx, api.LRPIdentifier{GUID: "idontknow", Version: "42"})
				Expect(err).To(MatchError(ContainSubstring("not found")))
			})
		})

		When("statefulsets cannot be listed", func() {
			BeforeEach(func() {
				statefulSetGetter.GetByLRPIdentifierReturns(nil, errors.New("who is this?"))
			})

			It("should return an error", func() {
				_, err := getter.Get(ctx, api.LRPIdentifier{GUID: "idontknow", Version: "42"})
				Expect(err).To(MatchError(ContainSubstring("failed to list statefulsets")))
			})
		})

		When("there are 2 lrps with the same identifier", func() {
			BeforeEach(func() {
				statefulSetGetter.GetByLRPIdentifierReturns([]appsv1.StatefulSet{{}, {}}, nil)
			})

			It("should return an error", func() {
				_, err := getter.Get(ctx, api.LRPIdentifier{GUID: "idontknow", Version: "42"})
				Expect(err).To(MatchError(ContainSubstring("multiple statefulsets found for LRP identifier")))
			})
		})
	})

	Describe("GetInstances", func() {
		BeforeEach(func() {
			statefulSetGetter.GetByLRPIdentifierReturns([]appsv1.StatefulSet{{}}, nil)
		})

		It("should list the correct pods", func() {
			pods := []corev1.Pod{
				{ObjectMeta: metav1.ObjectMeta{Name: "whatever-0"}},
				{ObjectMeta: metav1.ObjectMeta{Name: "whatever-1"}},
				{ObjectMeta: metav1.ObjectMeta{Name: "whatever-2"}},
			}
			podGetter.GetByLRPIdentifierReturns(pods, nil)
			eventGetter.GetByPodReturns([]corev1.Event{}, nil)

			_, err := getter.GetInstances(ctx, api.LRPIdentifier{GUID: "guid_1234", Version: "version_1234"})

			Expect(err).ToNot(HaveOccurred())
			Expect(podGetter.GetByLRPIdentifierCallCount()).To(Equal(1))
			_, lrpID := podGetter.GetByLRPIdentifierArgsForCall(0)
			Expect(lrpID).To(Equal(api.LRPIdentifier{GUID: "guid_1234", Version: "version_1234"}))
		})

		It("should return the correct number of instances", func() {
			pods := []corev1.Pod{
				{ObjectMeta: metav1.ObjectMeta{Name: "whatever-0"}},
				{ObjectMeta: metav1.ObjectMeta{Name: "whatever-1"}},
				{ObjectMeta: metav1.ObjectMeta{Name: "whatever-2"}},
			}
			podGetter.GetByLRPIdentifierReturns(pods, nil)
			eventGetter.GetByPodReturns([]corev1.Event{}, nil)
			instances, err := getter.GetInstances(ctx, api.LRPIdentifier{})
			Expect(err).ToNot(HaveOccurred())
			Expect(instances).To(HaveLen(3))
		})

		It("should return the correct instances information", func() {
			m := metav1.Unix(123, 0)
			pods := []corev1.Pod{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name: "whatever-1",
					},
					Status: corev1.PodStatus{
						StartTime: &m,
						Phase:     corev1.PodRunning,
						ContainerStatuses: []corev1.ContainerStatus{
							{
								State: corev1.ContainerState{Running: &corev1.ContainerStateRunning{}},
								Ready: true,
							},
						},
					},
				},
			}

			podGetter.GetByLRPIdentifierReturns(pods, nil)
			eventGetter.GetByPodReturns([]corev1.Event{}, nil)
			instances, err := getter.GetInstances(ctx, api.LRPIdentifier{GUID: "guid_1234", Version: "version_1234"})

			Expect(err).ToNot(HaveOccurred())
			Expect(instances).To(HaveLen(1))
			Expect(instances[0].Index).To(Equal(1))
			Expect(instances[0].Since).To(Equal(int64(123000000000)))
			Expect(instances[0].State).To(Equal("RUNNING"))
			Expect(instances[0].PlacementError).To(BeEmpty())
		})

		When("pod list fails", func() {
			It("should return a meaningful error", func() {
				podGetter.GetByLRPIdentifierReturns(nil, errors.New("boom"))

				_, err := getter.GetInstances(ctx, api.LRPIdentifier{})
				Expect(err).To(MatchError(ContainSubstring("failed to list pods")))
			})
		})

		When("the app does not exist", func() {
			It("should return an error", func() {
				statefulSetGetter.GetByLRPIdentifierReturns([]appsv1.StatefulSet{}, nil)

				_, err := getter.GetInstances(ctx, api.LRPIdentifier{GUID: "does-not", Version: "exist"})
				Expect(err).To(Equal(eirini.ErrNotFound))
			})
		})

		When("getting events fails", func() {
			It("should return a meaningful error", func() {
				pods := []corev1.Pod{
					{ObjectMeta: metav1.ObjectMeta{Name: "odin-0"}},
				}
				podGetter.GetByLRPIdentifierReturns(pods, nil)

				eventGetter.GetByPodReturns(nil, errors.New("I am error"))

				_, err := getter.GetInstances(ctx, api.LRPIdentifier{GUID: "guid_1234", Version: "version_1234"})
				Expect(err).To(MatchError(ContainSubstring(fmt.Sprintf("failed to get events for pod %s", "odin-0"))))
			})
		})

		When("time since creation is not available yet", func() {
			It("should return a default value", func() {
				pods := []corev1.Pod{
					{ObjectMeta: metav1.ObjectMeta{Name: "odin-0"}},
				}
				podGetter.GetByLRPIdentifierReturns(pods, nil)
				eventGetter.GetByPodReturns([]corev1.Event{}, nil)

				instances, err := getter.GetInstances(ctx, api.LRPIdentifier{})
				Expect(err).ToNot(HaveOccurred())
				Expect(instances).To(HaveLen(1))
				Expect(instances[0].Since).To(Equal(int64(0)))
			})
		})

		When("pods need too much resources", func() {
			BeforeEach(func() {
				pods := []corev1.Pod{
					{ObjectMeta: metav1.ObjectMeta{Name: "odin-0"}},
				}
				podGetter.GetByLRPIdentifierReturns(pods, nil)
			})

			When("the cluster has autoscaler", func() {
				BeforeEach(func() {
					eventGetter.GetByPodReturns([]corev1.Event{
						{
							Reason:  "NotTriggerScaleUp",
							Message: "pod didn't trigger scale-up (it wouldn't fit if a new node is added): 1 Insufficient memory",
						},
					}, nil)
				})

				It("returns insufficient memory response", func() {
					instances, err := getter.GetInstances(ctx, api.LRPIdentifier{})
					Expect(err).ToNot(HaveOccurred())
					Expect(instances).To(HaveLen(1))
					Expect(instances[0].PlacementError).To(Equal(api.InsufficientMemoryError))
				})
			})

			When("the cluster does not have autoscaler", func() {
				BeforeEach(func() {
					eventGetter.GetByPodReturns([]corev1.Event{
						{
							Reason:  "FailedScheduling",
							Message: "0/3 nodes are available: 3 Insufficient memory.",
						},
					}, nil)
				})

				It("returns insufficient memory response", func() {
					instances, err := getter.GetInstances(ctx, api.LRPIdentifier{})
					Expect(err).ToNot(HaveOccurred())
					Expect(instances).To(HaveLen(1))
					Expect(instances[0].PlacementError).To(Equal(api.InsufficientMemoryError))
				})
			})
		})

		When("the StatefulSet was deleted/stopped", func() {
			It("should return a default value", func() {
				event1 := corev1.Event{
					Reason: "Killing",
					InvolvedObject: corev1.ObjectReference{
						Name:      "odin-0",
						Namespace: namespace,
						UID:       "odin-0-uid",
					},
				}
				event2 := corev1.Event{
					Reason: "Killing",
					InvolvedObject: corev1.ObjectReference{
						Name:      "odin-1",
						Namespace: namespace,
						UID:       "odin-1-uid",
					},
				}
				eventGetter.GetByPodReturns([]corev1.Event{
					event1,
					event2,
				}, nil)

				pods := []corev1.Pod{
					{ObjectMeta: metav1.ObjectMeta{Name: "odin-0"}},
				}
				podGetter.GetByLRPIdentifierReturns(pods, nil)

				instances, err := getter.GetInstances(ctx, api.LRPIdentifier{})
				Expect(err).ToNot(HaveOccurred())
				Expect(instances).To(HaveLen(0))
			})
		})
	})
})

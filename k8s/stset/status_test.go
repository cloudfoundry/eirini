package stset_test

import (
	"code.cloudfoundry.org/eirini/api"
	"code.cloudfoundry.org/eirini/k8s/stset"
	"code.cloudfoundry.org/eirini/k8s/stset/stsetfakes"
	eiriniv1 "code.cloudfoundry.org/eirini/pkg/apis/eirini/v1"
	"code.cloudfoundry.org/lager"
	"code.cloudfoundry.org/lager/lagertest"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/pkg/errors"
	v1 "k8s.io/api/apps/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

var _ = Describe("StatusGetter", func() {
	var (
		logger            lager.Logger
		statefulSetGetter *stsetfakes.FakeStatefulSetByLRPIdentifierGetter
		statusGetter      stset.StatusGetter

		status eiriniv1.LRPStatus
		err    error
		lrpID  api.LRPIdentifier
	)

	BeforeEach(func() {
		logger = lagertest.NewTestLogger("status-getter-test")
		lrpID = api.LRPIdentifier{
			GUID:    "abc123",
			Version: "3",
		}
		statefulSetGetter = new(stsetfakes.FakeStatefulSetByLRPIdentifierGetter)

		statefulSetGetter.GetByLRPIdentifierReturns([]v1.StatefulSet{
			{
				ObjectMeta: metav1.ObjectMeta{
					Name: "foo",
				},
				Status: v1.StatefulSetStatus{
					ReadyReplicas: 42,
				},
			},
		}, nil)

		statusGetter = stset.NewStatusGetter(logger, statefulSetGetter)
	})

	JustBeforeEach(func() {
		status, err = statusGetter.GetStatus(ctx, lrpID)
	})

	It("succeeds", func() {
		Expect(err).NotTo(HaveOccurred())
	})

	It("gets the Status of the matching StatefulSet", func() {
		Expect(statefulSetGetter.GetByLRPIdentifierCallCount()).To(Equal(1))
		actualCtx, actualLRPId := statefulSetGetter.GetByLRPIdentifierArgsForCall(0)
		Expect(actualCtx).To(Equal(ctx))
		Expect(actualLRPId).To(Equal(lrpID))
		Expect(status.Replicas).To(Equal(int32(42)))
	})

	When("getting the StatefulSet fails", func() {
		BeforeEach(func() {
			statefulSetGetter.GetByLRPIdentifierReturns(nil, errors.New("get-by-lrp-id-error"))
		})

		It("fails", func() {
			Expect(err).To(MatchError(ContainSubstring("failed to get statefulset for LRP: failed to list statefulsets: get-by-lrp-id-error")))
		})
	})

	When("no statefulsets matching the LRP identifier exist", func() {
		BeforeEach(func() {
			statefulSetGetter.GetByLRPIdentifierReturns(nil, nil)
		})

		It("fails", func() {
			Expect(err).To(MatchError(ContainSubstring("failed to get statefulset for LRP: not found")))
		})
	})

	When("multiple statefulsets matching the LRP identifier exist", func() {
		BeforeEach(func() {
			statefulSetGetter.GetByLRPIdentifierReturns([]v1.StatefulSet{{}, {}}, nil)
		})

		It("fails", func() {
			Expect(err).To(MatchError(ContainSubstring("failed to get statefulset for LRP: multiple statefulsets found for LRP identifier")))
		})
	})
})

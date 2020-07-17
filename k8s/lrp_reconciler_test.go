package k8s_test

import (
	"context"

	"code.cloudfoundry.org/eirini"
	"code.cloudfoundry.org/eirini/k8s"
	"code.cloudfoundry.org/eirini/k8s/k8sfakes"
	"code.cloudfoundry.org/eirini/opi"
	eiriniv1 "code.cloudfoundry.org/eirini/pkg/apis/lrp/v1"
	lrpscheme "code.cloudfoundry.org/eirini/pkg/generated/clientset/versioned/scheme"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/pkg/errors"
	appsv1 "k8s.io/api/apps/v1"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

var _ = Describe("K8s/LrpReconciler", func() {

	var (
		controllerClient *k8sfakes.FakeClient
		desirer          *k8sfakes.FakeLRPDesirer
		scheme           *runtime.Scheme
		reconciler       *k8s.LRPReconciler
		resultErr        error
	)

	BeforeEach(func() {
		controllerClient = new(k8sfakes.FakeClient)
		desirer = new(k8sfakes.FakeLRPDesirer)
		scheme = lrpscheme.Scheme
		reconciler = k8s.NewLRPReconciler(controllerClient, desirer, scheme)

		controllerClient.GetStub = func(c context.Context, nn types.NamespacedName, o runtime.Object) error {
			lrp := o.(*eiriniv1.LRP)
			lrp.Name = "some-lrp"
			lrp.Namespace = "some-ns"
			lrp.Spec.GUID = "the-lrp-guid"
			lrp.Spec.Version = "the-lrp-version"
			lrp.Spec.Command = []string{"ls", "-la"}
			lrp.Spec.Instances = 10
			lrp.Spec.AppRoutes = []eiriniv1.Route{
				{Hostname: "foo.io", Port: 8080}, {Hostname: "bar.io", Port: 9090},
			}

			return nil
		}
		desirer.GetReturns(nil, eirini.ErrNotFound)
	})

	JustBeforeEach(func() {
		_, resultErr = reconciler.Reconcile(reconcile.Request{
			NamespacedName: types.NamespacedName{
				Namespace: "some-ns",
				Name:      "app",
			},
		})

	})
	It("creates a statefulset for each CRD", func() {
		Expect(resultErr).NotTo(HaveOccurred())

		Expect(desirer.UpdateCallCount()).To(Equal(0))
		Expect(desirer.DesireCallCount()).To(Equal(1))

		ns, lrp, _ := desirer.DesireArgsForCall(0)
		Expect(ns).To(Equal("some-ns"))
		Expect(lrp.GUID).To(Equal("the-lrp-guid"))
		Expect(lrp.Version).To(Equal("the-lrp-version"))
		Expect(lrp.Command).To(ConsistOf("ls", "-la"))
		Expect(lrp.TargetInstances).To(Equal(10))
		Expect(lrp.AppURIs).To(ConsistOf(
			opi.Route{Hostname: "foo.io", Port: 8080},
			opi.Route{Hostname: "bar.io", Port: 9090},
		))
	})

	It("sets an owner reference in the statefulset", func() {
		Expect(resultErr).NotTo(HaveOccurred())

		Expect(desirer.DesireCallCount()).To(Equal(1))
		_, _, setOwnerFns := desirer.DesireArgsForCall(0)
		Expect(setOwnerFns).To(HaveLen(1))
		setOwnerFn := setOwnerFns[0]

		st := &appsv1.StatefulSet{ObjectMeta: v1.ObjectMeta{Namespace: "some-ns"}}
		Expect(setOwnerFn(st)).To(Succeed())
		Expect(st.ObjectMeta.OwnerReferences).To(HaveLen(1))
		Expect(st.ObjectMeta.OwnerReferences[0].Kind).To(Equal("LRP"))
		Expect(st.ObjectMeta.OwnerReferences[0].Name).To(Equal("some-lrp"))
	})

	When("a CRD for this app already exists", func() {

		BeforeEach(func() {
			desirer.GetReturns(nil, nil)
		})

		It("updates it", func() {
			Expect(resultErr).NotTo(HaveOccurred())

			Expect(desirer.UpdateCallCount()).To(Equal(1))
			lrp := desirer.UpdateArgsForCall(0)
			Expect(lrp.TargetInstances).To(Equal(10))
			Expect(lrp.AppURIs).To(ConsistOf(
				opi.Route{Hostname: "foo.io", Port: 8080},
				opi.Route{Hostname: "bar.io", Port: 9090},
			))
		})

	})

	When("the controller client fails to get the CRD", func() {
		BeforeEach(func() {
			controllerClient.GetStub = func(c context.Context, nn types.NamespacedName, o runtime.Object) error {
				return errors.New("boom")
			}
		})

		It("returns an error", func() {
			Expect(resultErr).To(MatchError("boom"))
		})
	})

	When("the lrp desirer fails to get the app", func() {
		BeforeEach(func() {
			desirer.GetReturns(nil, errors.New("boom"))
		})

		It("returns an error", func() {
			Expect(resultErr).To(MatchError("failed to get lrp: boom"))
		})
	})

	When("the lrp desirer fails to desire the app", func() {
		BeforeEach(func() {
			desirer.DesireReturns(errors.New("boom"))
		})

		It("returns an error", func() {
			Expect(resultErr).To(MatchError("failed to desire lrp: boom"))
		})
	})

	When("the lrp desirer fails to update the app", func() {
		BeforeEach(func() {
			desirer.GetReturns(nil, nil)
			desirer.UpdateReturns(errors.New("boom"))
		})

		It("returns an error", func() {
			Expect(resultErr).To(MatchError("failed to update lrp: boom"))
		})
	})

})

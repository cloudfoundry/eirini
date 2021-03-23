package reconciler_test

import (
	"context"
	"fmt"

	"code.cloudfoundry.org/eirini/k8s/reconciler"
	"code.cloudfoundry.org/eirini/k8s/reconciler/reconcilerfakes"
	"code.cloudfoundry.org/eirini/opi"
	eiriniv1 "code.cloudfoundry.org/eirini/pkg/apis/eirini/v1"
	eiriniv1scheme "code.cloudfoundry.org/eirini/pkg/generated/clientset/versioned/scheme"
	"code.cloudfoundry.org/lager/lagertest"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	batchv1 "k8s.io/api/batch/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

var _ = Describe("Task", func() {
	var (
		taskReconciler   *reconciler.Task
		reconcileResult  reconcile.Result
		reconcileErr     error
		controllerClient *reconcilerfakes.FakeClient
		namespacedName   types.NamespacedName
		taskDesirer      *reconcilerfakes.FakeTaskDesirer
		scheme           *runtime.Scheme
	)

	BeforeEach(func() {
		controllerClient = new(reconcilerfakes.FakeClient)
		namespacedName = types.NamespacedName{
			Namespace: "my-namespace",
			Name:      "my-name",
		}
		taskDesirer = new(reconcilerfakes.FakeTaskDesirer)

		scheme = eiriniv1scheme.Scheme
		logger := lagertest.NewTestLogger("task-reconciler")
		taskReconciler = reconciler.NewTask(logger, controllerClient, taskDesirer, scheme)
	})

	JustBeforeEach(func() {
		reconcileResult, reconcileErr = taskReconciler.Reconcile(context.Background(), reconcile.Request{NamespacedName: namespacedName})
	})

	Context("creating a job", func() {
		BeforeEach(func() {
			controllerClient.GetStub = func(ctx context.Context, namespacedName types.NamespacedName, obj client.Object) error {
				task, ok := obj.(*eiriniv1.Task)
				Expect(ok).To(BeTrue())

				task.Name = namespacedName.Name
				task.Namespace = namespacedName.Namespace
				task.Spec.GUID = "my-task-guid"
				task.Spec.Name = "my-task-name"
				task.Spec.Image = "my-task-image"
				task.Spec.CompletionCallback = "my-task-completion-callback"
				task.Spec.PrivateRegistry = &eiriniv1.PrivateRegistry{
					Username: "pr-username",
					Password: "pr-password",
				}
				task.Spec.Env = map[string]string{"foo": "2", "bar": "coffee"}
				task.Spec.Command = []string{"beam", "me", "up"}
				task.Spec.AppName = "arthur"
				task.Spec.AppGUID = "arthur-guid"
				task.Spec.OrgName = "my-org"
				task.Spec.OrgGUID = "org-guid"
				task.Spec.SpaceName = "my-space"
				task.Spec.SpaceGUID = "space-guid"
				task.Spec.MemoryMB = 1234
				task.Spec.DiskMB = 4312
				task.Spec.CPUWeight = 14

				return nil
			}
		})

		It("creates the job in the CR's namespace", func() {
			Expect(reconcileErr).NotTo(HaveOccurred())

			By("looking up the task CR", func() {
				Expect(controllerClient.GetCallCount()).To(Equal(1))
				_, name, _ := controllerClient.GetArgsForCall(0)
				Expect(name).To(Equal(types.NamespacedName{
					Namespace: "my-namespace",
					Name:      "my-name",
				}))
			})

			By("invoking the task desirer", func() {
				Expect(taskDesirer.DesireCallCount()).To(Equal(1))
				_, namespace, opiTask, _ := taskDesirer.DesireArgsForCall(0)
				Expect(namespace).To(Equal("my-namespace"))
				Expect(opiTask.GUID).To(Equal("my-task-guid"))
				Expect(opiTask.Name).To(Equal("my-task-name"))
				Expect(opiTask.Image).To(Equal("my-task-image"))
				Expect(opiTask.CompletionCallback).To(Equal("my-task-completion-callback"))
				Expect(opiTask.PrivateRegistry).To(Equal(&opi.PrivateRegistry{
					Server:   "index.docker.io/v1/",
					Username: "pr-username",
					Password: "pr-password",
				}))
				Expect(opiTask.Env).To(Equal(map[string]string{"foo": "2", "bar": "coffee"}))
				Expect(opiTask.Command).To(Equal([]string{"beam", "me", "up"}))
				Expect(opiTask.AppName).To(Equal("arthur"))
				Expect(opiTask.AppGUID).To(Equal("arthur-guid"))
				Expect(opiTask.OrgName).To(Equal("my-org"))
				Expect(opiTask.OrgGUID).To(Equal("org-guid"))
				Expect(opiTask.SpaceName).To(Equal("my-space"))
				Expect(opiTask.SpaceGUID).To(Equal("space-guid"))
				Expect(opiTask.MemoryMB).To(BeNumerically("==", 1234))
				Expect(opiTask.DiskMB).To(BeNumerically("==", 4312))
				Expect(opiTask.CPUWeight).To(BeNumerically("==", 14))
			})

			By("sets an owner reference in the statefulset", func() {
				Expect(taskDesirer.DesireCallCount()).To(Equal(1))
				_, _, _, setOwnerFns := taskDesirer.DesireArgsForCall(0)
				Expect(setOwnerFns).To(HaveLen(1))
				setOwnerFn := setOwnerFns[0]

				job := &batchv1.Job{ObjectMeta: metav1.ObjectMeta{Namespace: "my-namespace"}}
				Expect(setOwnerFn(job)).To(Succeed())
				Expect(job.ObjectMeta.OwnerReferences).To(HaveLen(1))
				Expect(job.ObjectMeta.OwnerReferences[0].Kind).To(Equal("Task"))
				Expect(job.ObjectMeta.OwnerReferences[0].Name).To(Equal("my-name"))
			})
		})
	})

	When("the task cannot be found", func() {
		BeforeEach(func() {
			controllerClient.GetReturns(errors.NewNotFound(schema.GroupResource{}, "foo"))
		})

		It("neither requeues nor returns an error", func() {
			Expect(reconcileResult.Requeue).To(BeFalse())
			Expect(reconcileErr).ToNot(HaveOccurred())
		})
	})

	When("getting the task returns another error", func() {
		BeforeEach(func() {
			controllerClient.GetReturns(fmt.Errorf("some problem"))
		})

		It("returns an error", func() {
			Expect(reconcileErr).To(MatchError(ContainSubstring("some problem")))
		})
	})

	When("desiring the task returns an error", func() {
		BeforeEach(func() {
			taskDesirer.DesireReturns(fmt.Errorf("some error"))
		})

		It("returns an error", func() {
			Expect(reconcileErr).To(MatchError(ContainSubstring("some error")))
		})
	})

	When("the task already exists", func() {
		BeforeEach(func() {
			taskDesirer.DesireReturns(errors.NewAlreadyExists(schema.GroupResource{}, "the-task"))
		})

		It("does not error or requeue", func() {
			Expect(reconcileResult.Requeue).To(BeFalse())
			Expect(reconcileErr).ToNot(HaveOccurred())
		})
	})
})

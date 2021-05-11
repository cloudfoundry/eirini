package reconciler_test

import (
	"context"
	"fmt"

	"code.cloudfoundry.org/eirini/k8s/reconciler"
	"code.cloudfoundry.org/eirini/k8s/reconciler/reconcilerfakes"
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
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

var _ = Describe("Task", func() {
	var (
		taskReconciler  *reconciler.Task
		reconcileResult reconcile.Result
		reconcileErr    error
		taskClient      *reconcilerfakes.FakeTasksRuntimeClient
		namespacedName  types.NamespacedName
		workloadClient  *reconcilerfakes.FakeWorkloadClient
		scheme          *runtime.Scheme
		task            *eiriniv1.Task
	)

	BeforeEach(func() {
		taskClient = new(reconcilerfakes.FakeTasksRuntimeClient)
		namespacedName = types.NamespacedName{
			Namespace: "my-namespace",
			Name:      "my-name",
		}
		workloadClient = new(reconcilerfakes.FakeWorkloadClient)

		scheme = eiriniv1scheme.Scheme
		logger := lagertest.NewTestLogger("task-reconciler")
		taskReconciler = reconciler.NewTask(logger, taskClient, workloadClient, scheme)
		task = &eiriniv1.Task{
			ObjectMeta: metav1.ObjectMeta{
				Name:      namespacedName.Name,
				Namespace: namespacedName.Namespace,
			},
			Spec: eiriniv1.TaskSpec{
				GUID:               "guid",
				Name:               "my-name",
				Image:              "my/image",
				CompletionCallback: "callback",
				Env:                map[string]string{"foo": "bar"},
				Command:            []string{"foo", "baz"},
				AppName:            "jim",
				AppGUID:            "app-guid",
				OrgName:            "organ",
				OrgGUID:            "orgid",
				SpaceName:          "spacan",
				SpaceGUID:          "spacid",
				MemoryMB:           768,
				DiskMB:             512,
				CPUWeight:          13,
			},
		}
		taskClient.GetTaskReturns(task, nil)
		workloadClient.GetStatusReturns(eiriniv1.TaskStatus{
			ExecutionStatus: eiriniv1.TaskStarting,
		}, nil)
	})

	JustBeforeEach(func() {
		reconcileResult, reconcileErr = taskReconciler.Reconcile(context.Background(), reconcile.Request{NamespacedName: namespacedName})
	})

	It("creates the job in the CR's namespace", func() {
		Expect(reconcileErr).NotTo(HaveOccurred())

		By("invoking the task desirer", func() {
			Expect(workloadClient.DesireCallCount()).To(Equal(1))
			_, namespace, apiTask, _ := workloadClient.DesireArgsForCall(0)
			Expect(namespace).To(Equal("my-namespace"))
			Expect(apiTask.GUID).To(Equal(task.Spec.GUID))
			Expect(apiTask.Name).To(Equal(task.Spec.Name))
			Expect(apiTask.Image).To(Equal(task.Spec.Image))
			Expect(apiTask.CompletionCallback).To(Equal(task.Spec.CompletionCallback))
			Expect(apiTask.Env).To(Equal(task.Spec.Env))
			Expect(apiTask.Command).To(Equal(task.Spec.Command))
			Expect(apiTask.AppName).To(Equal(task.Spec.AppName))
			Expect(apiTask.AppGUID).To(Equal(task.Spec.AppGUID))
			Expect(apiTask.OrgName).To(Equal(task.Spec.OrgName))
			Expect(apiTask.OrgGUID).To(Equal(task.Spec.OrgGUID))
			Expect(apiTask.SpaceName).To(Equal(task.Spec.SpaceName))
			Expect(apiTask.SpaceGUID).To(Equal(task.Spec.SpaceGUID))
			Expect(apiTask.MemoryMB).To(Equal(task.Spec.MemoryMB))
			Expect(apiTask.DiskMB).To(Equal(task.Spec.DiskMB))
			Expect(apiTask.CPUWeight).To(Equal(task.Spec.CPUWeight))
		})

		By("updating the task execution status", func() {
			Expect(taskClient.UpdateTaskStatusCallCount()).To(Equal(1))
			_, actualTask, status := taskClient.UpdateTaskStatusArgsForCall(0)
			Expect(actualTask).To(Equal(task))
			Expect(status.ExecutionStatus).To(Equal(eiriniv1.TaskStarting))
		})

		By("sets an owner reference in the job", func() {
			Expect(workloadClient.DesireCallCount()).To(Equal(1))
			_, _, _, setOwnerFns := workloadClient.DesireArgsForCall(0)
			Expect(setOwnerFns).To(HaveLen(1))
			setOwnerFn := setOwnerFns[0]

			job := &batchv1.Job{ObjectMeta: metav1.ObjectMeta{Namespace: "my-namespace"}}
			Expect(setOwnerFn(job)).To(Succeed())
			Expect(job.ObjectMeta.OwnerReferences).To(HaveLen(1))
			Expect(job.ObjectMeta.OwnerReferences[0].Kind).To(Equal("Task"))
			Expect(job.ObjectMeta.OwnerReferences[0].Name).To(Equal("my-name"))
		})
	})

	It("loads the task using names from request", func() {
		Expect(taskClient.GetTaskCallCount()).To(Equal(1))
		_, namespace, name := taskClient.GetTaskArgsForCall(0)
		Expect(namespace).To(Equal("my-namespace"))
		Expect(name).To(Equal("my-name"))
	})

	When("the task cannot be found", func() {
		BeforeEach(func() {
			taskClient.GetTaskReturns(nil, errors.NewNotFound(schema.GroupResource{}, "foo"))
		})

		It("neither requeues nor returns an error", func() {
			Expect(reconcileResult.Requeue).To(BeFalse())
			Expect(reconcileErr).ToNot(HaveOccurred())
		})
	})

	When("getting the task returns another error", func() {
		BeforeEach(func() {
			taskClient.GetTaskReturns(nil, fmt.Errorf("some problem"))
		})

		It("returns an error", func() {
			Expect(reconcileErr).To(MatchError(ContainSubstring("some problem")))
		})
	})

	It("gets the new task status", func() {
		Expect(workloadClient.GetStatusCallCount()).To(Equal(1))
		_, guid := workloadClient.GetStatusArgsForCall(0)
		Expect(guid).To(Equal("guid"))
	})

	It("updates the task with the new status", func() {
		Expect(taskClient.UpdateTaskStatusCallCount()).To(Equal(1))
		_, _, newStatus := taskClient.UpdateTaskStatusArgsForCall(0)
		Expect(newStatus.ExecutionStatus).To(Equal(eiriniv1.TaskStarting))
	})

	When("gettin the task status returns an error", func() {
		BeforeEach(func() {
			workloadClient.GetStatusReturns(eiriniv1.TaskStatus{}, fmt.Errorf("potato"))
		})

		It("returns an error", func() {
			Expect(reconcileErr).To(MatchError(ContainSubstring("potato")))
		})
	})

	When("updating the task status returns an error", func() {
		BeforeEach(func() {
			taskClient.UpdateTaskStatusReturns(fmt.Errorf("crumpets"))
		})

		It("returns an error", func() {
			Expect(reconcileErr).To(MatchError(ContainSubstring("crumpets")))
		})
	})

	When("there is a private registry set", func() {
		BeforeEach(func() {
			task.Spec.PrivateRegistry = &eiriniv1.PrivateRegistry{
				Username: "admin",
				Password: "p4ssw0rd",
			}
		})

		It("passes the private registry details to the desirer", func() {
			Expect(workloadClient.DesireCallCount()).To(Equal(1))
			_, _, apiTask, _ := workloadClient.DesireArgsForCall(0)
			Expect(apiTask.PrivateRegistry).ToNot(BeNil())
			Expect(apiTask.PrivateRegistry.Username).To(Equal("admin"))
			Expect(apiTask.PrivateRegistry.Password).To(Equal("p4ssw0rd"))
			Expect(apiTask.PrivateRegistry.Server).To(Equal("index.docker.io/v1/"))
		})
	})

	When("desiring the task returns an error", func() {
		BeforeEach(func() {
			workloadClient.DesireReturns(fmt.Errorf("some error"))
		})

		It("returns an error", func() {
			Expect(reconcileErr).To(MatchError(ContainSubstring("some error")))
			Expect(taskClient.UpdateTaskStatusCallCount()).To(Equal(0))
		})
	})
})

package k8s_test

import (
	"context"
	"os"
	"path/filepath"
	"time"

	"github.com/julz/cube/k8s"
	"github.com/julz/cube/opi"
	"k8s.io/api/core/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	bv1 "k8s.io/api/batch/v1"
	av1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

var _ = Describe("Desiretask", func() {
	var (
		client    *kubernetes.Clientset
		desirer   *k8s.TaskDesirer
		namespace string
		tasks     []opi.Task
	)

	namespaceExists := func(name string) bool {
		_, err := client.CoreV1().Namespaces().Get(namespace, av1.GetOptions{})
		return err == nil
	}

	createNamespace := func(name string) {
		namespaceSpec := &v1.Namespace{
			ObjectMeta: av1.ObjectMeta{Name: name},
		}

		if _, err := client.CoreV1().Namespaces().Create(namespaceSpec); err != nil {
			panic(err)
		}
	}

	getTaskNames := func() []string {
		names := []string{}
		for _, task := range tasks {
			names = append(names, task.Env["APP_ID"])
		}
		return names
	}

	listJobs := func() []bv1.Job {
		list, err := client.BatchV1().Jobs(namespace).List(av1.ListOptions{})
		Expect(err).NotTo(HaveOccurred())
		return list.Items
	}

	BeforeEach(func() {
		config, err := clientcmd.BuildConfigFromFlags("", filepath.Join(os.Getenv("HOME"), ".kube", "config"))
		if err != nil {
			panic(err.Error())
		}

		client, err = kubernetes.NewForConfig(config)
		if err != nil {
			panic(err.Error())
		}

		namespace = "testing"
		tasks = []opi.Task{
			{Image: "pi", Command: []string{}, Env: map[string]string{"APP_ID": "guid0", "STAGING_GUID": "guid0"}},
			{Image: "pi", Command: []string{}, Env: map[string]string{"APP_ID": "guid1", "STAGING_GUID": "guid1"}},
		}
	})

	JustBeforeEach(func() {
		if !namespaceExists(namespace) {
			createNamespace(namespace)
		}

		desirer = &k8s.TaskDesirer{
			Config: k8s.JobConfig{Namespace: namespace},
			Client: client,
		}
	})

	Context("When desiring some tasks", func() {

		AfterEach(func() {
			for _, jobName := range getTaskNames() {
				if err := client.BatchV1().Jobs(namespace).Delete(jobName, &av1.DeleteOptions{}); err != nil {
					panic(err)
				}
			}

			Eventually(listJobs, 5*time.Second).Should(BeEmpty())
		})

		getJobNames := func(jobs *bv1.JobList) []string {
			jobNames := []string{}
			for _, job := range jobs.Items {
				jobNames = append(jobNames, job.ObjectMeta.Name)
			}

			return jobNames
		}

		It("creates jobs for every task in the array", func() {
			Expect(desirer.Desire(context.Background(), tasks)).To(Succeed())
			jobs, err := client.BatchV1().Jobs(namespace).List(av1.ListOptions{})

			Expect(err).NotTo(HaveOccurred())
			Expect(jobs.Items).To(HaveLen(len(tasks)))
			Expect(getJobNames(jobs)).To(ConsistOf(getTaskNames()))
		})
	})

	Context("When trying to delete some tasks", func() {

		It("deletes every task in the array", func() {
			Expect(desirer.Desire(context.Background(), tasks)).To(Succeed())

			for _, taskName := range getTaskNames() {
				Expect(desirer.DeleteJob(taskName)).To(Succeed())
			}

			Eventually(listJobs, 5*time.Second).Should(BeEmpty())
		})
	})
})

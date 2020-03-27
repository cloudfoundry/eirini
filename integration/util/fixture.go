package util

import (
	"bufio"
	"fmt"
	"io"
	"os"

	"github.com/hashicorp/go-multierror"
	"github.com/pkg/errors"

	corev1 "k8s.io/api/core/v1"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
)

type Fixture struct {
	Clientset      kubernetes.Interface
	Namespace      string
	PspName        string
	KubeConfigPath string
	Writer         io.Writer
}

func NewFixture(writer io.Writer) (Fixture, error) {
	kubeConfigPath := os.Getenv("INTEGRATION_KUBECONFIG")
	if kubeConfigPath == "" {
		return Fixture{}, errors.New("INTEGRATION_KUBECONFIG is not provided")
	}

	config, err := clientcmd.BuildConfigFromFlags("", kubeConfigPath)
	if err != nil {
		return Fixture{}, errors.Wrap(err, "failed to build config from flags")
	}

	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		return Fixture{}, errors.Wrap(err, "failed to create clientset")
	}

	return Fixture{
		KubeConfigPath: kubeConfigPath,
		Clientset:      clientset,
		Writer:         writer,
	}, nil
}

func (f Fixture) SetUp() (Fixture, error) {
	namespace := CreateRandomNamespace(f.Clientset)
	pspName := fmt.Sprintf("%s-psp", namespace)
	if err := CreatePodCreationPSP(namespace, pspName, f.Clientset); err != nil {
		return Fixture{}, errors.Wrap(err, "failed to create pod creation PSP")
	}
	return Fixture{
		KubeConfigPath: f.KubeConfigPath,
		Clientset:      f.Clientset,
		Namespace:      namespace,
		Writer:         f.Writer,
		PspName:        pspName,
	}, nil
}

func (f Fixture) TearDown() error {
	var errs *multierror.Error
	errs = multierror.Append(errs, f.printDebugInfo())

	errs = multierror.Append(errs, DeleteNamespace(f.Namespace, f.Clientset))
	errs = multierror.Append(errs, DeletePSP(f.PspName, f.Clientset))

	return errs.ErrorOrNil()
}

//nolint:gocyclo
func (f Fixture) printDebugInfo() error {
	if _, err := f.Writer.Write([]byte("Jobs:\n")); err != nil {
		return err
	}
	jobs, _ := f.Clientset.BatchV1().Jobs(f.Namespace).List(v1.ListOptions{})
	for _, job := range jobs.Items {
		fmt.Fprintf(f.Writer, "Job: %s status is: %#v\n", job.Name, job.Status)
		if _, err := f.Writer.Write([]byte("-----------\n")); err != nil {
			return err
		}
	}

	statefulsets, _ := f.Clientset.AppsV1().StatefulSets(f.Namespace).List(v1.ListOptions{})
	if _, err := f.Writer.Write([]byte("StatefulSets:\n")); err != nil {
		return err
	}
	for _, s := range statefulsets.Items {
		fmt.Fprintf(f.Writer, "StatefulSet: %s status is: %#v\n", s.Name, s.Status)
		if _, err := f.Writer.Write([]byte("-----------\n")); err != nil {
			return err
		}
	}

	pods, _ := f.Clientset.CoreV1().Pods(f.Namespace).List(v1.ListOptions{})
	if _, err := f.Writer.Write([]byte("Pods:\n")); err != nil {
		return err
	}
	for _, p := range pods.Items {
		fmt.Fprintf(f.Writer, "Pod: %s status is: %#v\n", p.Name, p.Status)
		if _, err := f.Writer.Write([]byte("-----------\n")); err != nil {
			return err
		}
		fmt.Fprintf(f.Writer, "Pod: %s logs are: \n", p.Name)
		logsReq := f.Clientset.CoreV1().Pods(f.Namespace).GetLogs(p.Name, &corev1.PodLogOptions{})
		if err := consumeRequest(logsReq, f.Writer); err != nil {
			fmt.Fprintf(f.Writer, "Failed to get logs for Pod: %s becase: %v \n", p.Name, err)
		}
	}

	return nil
}

func consumeRequest(request rest.ResponseWrapper, out io.Writer) error {
	readCloser, err := request.Stream()
	if err != nil {
		return err
	}
	defer readCloser.Close()

	r := bufio.NewReader(readCloser)
	for {
		bytes, err := r.ReadBytes('\n')
		if _, writeErr := out.Write(bytes); writeErr != nil {
			return writeErr
		}

		if err != nil {
			if err != io.EOF {
				return err
			}
			return nil
		}
	}
}

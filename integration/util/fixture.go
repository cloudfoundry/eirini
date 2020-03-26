package util

import (
	"bufio"
	"fmt"
	"io"
	"os"

	"github.com/hashicorp/go-multierror"
	"github.com/pkg/errors"

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
	f.Writer.Write([]byte("Jobs:"))
	jobs, _ := f.Clientset.BatchV1().Jobs(f.Namespace).List(v1.ListOptions{})
	for _, job := range jobs.Items {
		fmt.Fprintf(f.Writer, "Job: %s status is: %#v\n", job.Name, job.Status)
		f.Writer.Write([]byte("-----------\n"))
	}

	statefulsets, _ := f.Clientset.AppsV1().StatefulSets(f.Namespace).List(v1.ListOptions{})
	f.Writer.Write([]byte("StatefulSets:"))
	for _, s := range statefulsets.Items {
		fmt.Fprintf(f.Writer, "StatefulSet: %s status is: %#v\n", s.Name, s.Status)
		f.Writer.Write([]byte("-----------\n"))
	}

	pods, _ := f.Clientset.CoreV1().Pods(f.Namespace).List(v1.ListOptions{})
	f.Writer.Write([]byte("Pods:"))
	for _, p := range pods.Items {
		fmt.Fprintf(f.Writer, "Pod: %s status is: %#v\n", p.Name, p.Status)
		f.Writer.Write([]byte("-----------\n"))
		logsReq := f.Clientset.CoreV1().Pods(f.Namespace).GetLogs(p.Name, nil)
		fmt.Fprintf(f.Writer, "Pod: %s logs are: \n", p.Name)
		consumeRequest(logsReq, f.Writer)
	}

	errs = multierror.Append(errs, DeleteNamespace(f.Namespace, f.Clientset))
	errs = multierror.Append(errs, DeletePSP(f.PspName, f.Clientset))

	return errs.ErrorOrNil()
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
		if _, err := out.Write(bytes); err != nil {
			return err
		}

		if err != nil {
			if err != io.EOF {
				return err
			}
			return nil
		}
	}
}

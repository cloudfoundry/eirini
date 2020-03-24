package util

import (
	"fmt"
	"os"

	"github.com/hashicorp/go-multierror"
	"github.com/pkg/errors"

	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"
)

type Fixture struct {
	Clientset      kubernetes.Interface
	Namespace      string
	PspName        string
	KubeConfigPath string
}

func NewFixture() (Fixture, error) {
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
		PspName:        pspName,
	}, nil
}

func (f Fixture) TearDown() error {
	var errs *multierror.Error
	errs = multierror.Append(errs, DeleteNamespace(f.Namespace, f.Clientset))
	errs = multierror.Append(errs, DeletePSP(f.PspName, f.Clientset))

	return errs.ErrorOrNil()
}

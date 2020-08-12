package kubeconfig

import (
	"fmt"

	"go.uber.org/zap"
	"k8s.io/client-go/discovery"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
)

// Checker is the interface that wraps the Check method that checks if cfg can be used to connect to
// the Kubernetes cluster.
type Checker interface {
	Check(cfg *rest.Config) error
}

// NewChecker constructs a default checker that satisfies the Checker interface.
func NewChecker(log *zap.SugaredLogger) Checker {
	return &checker{
		log: log,

		createClientSet:    kubernetes.NewForConfig,
		checkServerVersion: checkServerVersion,
	}
}

type checker struct {
	log *zap.SugaredLogger

	createClientSet    func(c *rest.Config) (*kubernetes.Clientset, error)
	checkServerVersion func(d discovery.ServerVersionInterface) error
}

func (c *checker) Check(cfg *rest.Config) error {
	c.log.Info("Checking kube config")
	clientset, err := c.createClientSet(cfg)
	if err != nil {
		return &checkConfigError{err}
	}
	err = c.checkServerVersion(clientset.Discovery())
	if err != nil {
		return &checkConfigError{err}
	}
	return nil
}

type checkConfigError struct {
	err error
}

func (e *checkConfigError) Error() string {
	return fmt.Sprintf("invalid kube config: %v", e.err)
}

func checkServerVersion(d discovery.ServerVersionInterface) error {
	_, err := d.ServerVersion()
	return err
}

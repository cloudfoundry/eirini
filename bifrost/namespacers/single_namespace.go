package namespacers

import "fmt"

type SingleNamespace struct {
	defaultNamespace string
}

func NewSingleNamespace(defaultNs string) SingleNamespace {
	return SingleNamespace{
		defaultNamespace: defaultNs,
	}
}

func (n SingleNamespace) GetNamespace(ns string) (string, error) {
	if ns == "" {
		return n.defaultNamespace, nil
	}

	if ns != n.defaultNamespace {
		return "", fmt.Errorf("namespace %q is not allowed", ns)
	}

	return n.defaultNamespace, nil
}

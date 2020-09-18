package namespacers

type MultiNamespace struct {
	defaultNamespace string
}

func NewMultiNamespace(defaultNs string) MultiNamespace {
	return MultiNamespace{
		defaultNamespace: defaultNs,
	}
}

func (n MultiNamespace) GetNamespace(ns string) (string, error) {
	if ns != "" {
		return ns, nil
	}

	return n.defaultNamespace, nil
}

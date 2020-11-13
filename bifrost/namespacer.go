package bifrost

type Namespacer struct {
	defaultNamespace string
}

func NewNamespacer(defaultNs string) Namespacer {
	return Namespacer{
		defaultNamespace: defaultNs,
	}
}

func (n Namespacer) GetNamespace(ns string) string {
	if ns != "" {
		return ns
	}

	return n.defaultNamespace
}

package kubelet

import (
	"context"
	"encoding/json"

	"github.com/pkg/errors"
	"k8s.io/client-go/rest"
)

type Client struct {
	kubeClient rest.Interface
}

func NewClient(kubeClient rest.Interface) Client {
	return Client{
		kubeClient: kubeClient,
	}
}

func (c Client) StatsSummary(nodename string) (StatsSummary, error) {
	var summary StatsSummary
	result := c.kubeClient.
		Get().
		Resource("nodes").
		Name(nodename).
		SubResource("proxy", "stats", "summary").
		Do(context.Background())
	body, err := result.Raw()
	if err != nil {
		return summary, errors.Wrap(err, "failed to get raw body")
	}
	err = json.Unmarshal(body, &summary)
	if err != nil {
		return summary, errors.Wrap(err, "failed to unmarshal body")
	}

	return summary, nil
}

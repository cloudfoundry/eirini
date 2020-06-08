package controller

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"

	"code.cloudfoundry.org/eirini/models/cf"
	eiriniv1 "code.cloudfoundry.org/eirini/pkg/apis/lrp/v1"
	"github.com/jinzhu/copier"
	"github.com/pkg/errors"
)

type RestLrp struct {
	client    *http.Client
	eiriniURI string
}

func NewRestLrp(client *http.Client, eiriniURI string) *RestLrp {
	return &RestLrp{
		client:    client,
		eiriniURI: eiriniURI,
	}
}

func (c RestLrp) Create(lrp eiriniv1.LRP) error {
	var createReq cf.DesireLRPRequest
	if err := copier.Copy(&createReq, &lrp.Spec); err != nil {
		return err
	}

	createReq.Namespace = lrp.Namespace

	requestURL := fmt.Sprintf("%s/apps/%s", c.eiriniURI, lrp.Spec.ProcessGUID)
	return errors.Wrapf(
		c.httpDo(http.MethodPut, requestURL, createReq),
		"failed to create lrp %s",
		lrp.Spec.ProcessGUID,
	)
}

func (c RestLrp) Update(oldLRP, lrp eiriniv1.LRP) error {
	if oldLRP.Spec.LastUpdated == lrp.Spec.LastUpdated {
		return nil
	}

	updateReq := cf.UpdateDesiredLRPRequest{
		GUID:    lrp.Spec.GUID,
		Version: lrp.Spec.Version,
		Update: cf.DesiredLRPUpdate{
			Instances:  lrp.Spec.NumInstances,
			Routes:     lrp.Spec.Routes,
			Annotation: lrp.Spec.LastUpdated,
		},
	}

	requestURL := fmt.Sprintf("%s/apps/%s", c.eiriniURI, lrp.Spec.ProcessGUID)
	return errors.Wrapf(
		c.httpDo(http.MethodPost, requestURL, updateReq),
		"failed to update lrp %s",
		lrp.Spec.ProcessGUID,
	)
}

func (c RestLrp) Delete(lrp eiriniv1.LRP) error {
	requestURL := fmt.Sprintf("%s/apps/%s/%s/stop", c.eiriniURI, lrp.Spec.GUID, lrp.Spec.Version)
	return errors.Wrapf(
		c.httpDo(http.MethodPut, requestURL, nil),
		"failed to delete lrp %s",
		lrp.Spec.ProcessGUID,
	)
}

func (c RestLrp) httpDo(method, requestURL string, data interface{}) error {
	body, err := json.Marshal(data)
	if err != nil {
		return errors.Wrap(err, "failed to marshal lrp request")
	}
	req, err := http.NewRequest(method,
		requestURL,
		bytes.NewReader(body),
	)
	if err != nil {
		return errors.Wrap(err, "failed to create http request")
	}

	req.Header.Set("Content-Type", "application/json")
	response, err := c.client.Do(req)
	if err != nil {
		return errors.Wrap(err, "failed to send http request")
	}
	defer response.Body.Close()

	if response.StatusCode > http.StatusBadRequest {
		return fmt.Errorf("status code: %d", response.StatusCode)
	}

	return nil
}

package main

import (
	"code.cloudfoundry.org/eirini"
	"github.com/pkg/errors"
)

type Uploader struct {
	Cfclient eirini.CfClient
}

func (u *Uploader) Upload(guid string, path string) error {
	if guid == "" {
		return errors.New("empty guid parameter")
	}

	if path == "" {
		return errors.New("empty path parameter")
	}

	err := u.Cfclient.PushDroplet(path, guid)
	if err != nil {
		return errors.Wrap(err, "perform request failed")
	}

	return nil
}

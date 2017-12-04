package controllers

import (
	"code.cloudfoundry.org/bbs/models"
	"code.cloudfoundry.org/lager"
)

func exitIfUnrecoverableError(logger lager.Logger, err error, exitChan chan<- struct{}) {
	bbsErr := models.ConvertError(err)
	if bbsErr != nil && bbsErr.GetType() == models.Error_Unrecoverable {
		select {
		case exitChan <- struct{}{}:
		default:
		}
	}
}

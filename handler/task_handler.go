package handler

import (
	"net/http"

	"code.cloudfoundry.org/eirini"
	"code.cloudfoundry.org/lager"
	"github.com/julienschmidt/httprouter"
)

type Task struct {
	logger  lager.Logger
	desirer eirini.TaskDesirer
}

func NewTaskHandler(logger lager.Logger, desirer eirini.TaskDesirer) *Task {
	return &Task{
		logger:  logger,
		desirer: desirer,
	}
}

func (s *Task) Run(resp http.ResponseWriter, req *http.Request, ps httprouter.Params) {
	resp.WriteHeader(http.StatusAccepted)
}

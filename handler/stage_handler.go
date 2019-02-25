package handler

import (
	"encoding/json"
	"net/http"

	"code.cloudfoundry.org/bbs/models"
	"code.cloudfoundry.org/eirini"
	"code.cloudfoundry.org/eirini/models/cf"
	"code.cloudfoundry.org/lager"
	"github.com/julienschmidt/httprouter"
)

type Stage struct {
	stager eirini.Stager
	logger lager.Logger
}

func NewStageHandler(stager eirini.Stager, logger lager.Logger) *Stage {
	logger = logger.Session("staging-handler")

	return &Stage{
		stager: stager,
		logger: logger,
	}
}

func (s *Stage) Stage(resp http.ResponseWriter, req *http.Request, ps httprouter.Params) {
	stagingGUID := ps.ByName("staging_guid")
	logger := s.logger.Session("staging-request", lager.Data{"staging-guid": stagingGUID})

	var stagingRequest cf.StagingRequest
	if err := json.NewDecoder(req.Body).Decode(&stagingRequest); err != nil {
		logger.Error("staging-request-body-decoding-failed", err)
		writeErrorResponse(resp, http.StatusBadRequest, err)
		return
	}

	if err := s.stager.Stage(stagingGUID, stagingRequest); err != nil {
		logger.Error("stage-app-failed", err)
		writeErrorResponse(resp, http.StatusInternalServerError, err)
		return
	}

	resp.WriteHeader(http.StatusAccepted)
}

func (s *Stage) StagingComplete(res http.ResponseWriter, req *http.Request, ps httprouter.Params) {
	stagingGUID := ps.ByName("staging_guid")
	logger := s.logger.Session("staging-complete", lager.Data{"staging-guid": stagingGUID})

	task := &models.TaskCallbackResponse{}
	err := json.NewDecoder(req.Body).Decode(task)
	if err != nil {
		logger.Error("parsing-incoming-task-failed", err)
		res.WriteHeader(http.StatusBadRequest)
		return
	}

	if err = s.stager.CompleteStaging(task); err != nil {
		res.WriteHeader(http.StatusInternalServerError)
		logger.Error("staging-completion-failed", err)
		return
	}

	logger.Info("posted-staging-complete")
}

func writeErrorResponse(resp http.ResponseWriter, status int, err error) {
	resp.WriteHeader(status)
	encodingErr := json.NewEncoder(resp).Encode(&cf.StagingError{Message: err.Error()})
	if encodingErr != nil {
		panic(encodingErr)
	}
}

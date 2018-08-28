package handler

import (
	"encoding/json"
	"net/http"

	"code.cloudfoundry.org/bbs/models"
	"code.cloudfoundry.org/eirini"
	"code.cloudfoundry.org/lager"
	"code.cloudfoundry.org/runtimeschema/cc_messages"
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

	var stagingRequest cc_messages.StagingRequestFromCC
	if err := json.NewDecoder(req.Body).Decode(&stagingRequest); err != nil {
		s.logger.Error("staging-request-body-decoding-failed", err)
		resp.WriteHeader(http.StatusBadRequest)
		return
	}

	if err := s.stager.Stage(stagingGUID, stagingRequest); err != nil {
		logger.Error("stage-app-failed", err)
		resp.WriteHeader(http.StatusInternalServerError)
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

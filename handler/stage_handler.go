package handler

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"

	"code.cloudfoundry.org/bbs/models"
	"code.cloudfoundry.org/eirini/models/cf"
	"code.cloudfoundry.org/lager"
	"github.com/julienschmidt/httprouter"
	"github.com/pkg/errors"
)

type Stage struct {
	buildpackStager StagingBifrost
	dockerStager    StagingBifrost
	logger          lager.Logger
}

func NewStageHandler(buildpackStager, dockerStager StagingBifrost, logger lager.Logger) *Stage {
	logger = logger.Session("staging-handler")

	return &Stage{
		buildpackStager: buildpackStager,
		dockerStager:    dockerStager,
		logger:          logger,
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

	if err := s.stage(stagingGUID, stagingRequest); err != nil {
		reason := fmt.Sprintf("staging task with guid %s failed to start", stagingGUID)
		logger.Error("staging-failed", errors.Wrap(err, reason))
		writeErrorResponse(resp, http.StatusInternalServerError, errors.New(reason))
		return
	}

	resp.WriteHeader(http.StatusAccepted)
}

func (s *Stage) stage(stagingGUID string, stagingRequest cf.StagingRequest) error {
	if stagingRequest.Lifecycle.DockerLifecycle != nil {
		return s.dockerStager.TransferStaging(context.Background(), stagingGUID, stagingRequest)
	}
	return s.buildpackStager.TransferStaging(context.Background(), stagingGUID, stagingRequest)
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

	if err = s.buildpackStager.CompleteStaging(task); err != nil {
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

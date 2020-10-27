package handler

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"

	"code.cloudfoundry.org/eirini/models/cf"
	"code.cloudfoundry.org/lager"
	"github.com/julienschmidt/httprouter"
	"github.com/pkg/errors"
)

type Stage struct {
	buildpackStagingBifrost StagingBifrost
	dockerStagingBifrost    StagingBifrost
	logger                  lager.Logger
}

func NewStageHandler(buildpackStagingBifrost, dockerStagingBifrost StagingBifrost, logger lager.Logger) *Stage {
	logger = logger.Session("staging-handler")

	return &Stage{
		buildpackStagingBifrost: buildpackStagingBifrost,
		dockerStagingBifrost:    dockerStagingBifrost,
		logger:                  logger,
	}
}

func (s *Stage) Run(resp http.ResponseWriter, req *http.Request, ps httprouter.Params) {
	stagingGUID := ps.ByName("staging_guid")
	logger := s.logger.Session("staging-request", lager.Data{"staging-guid": stagingGUID})

	var stagingRequest cf.StagingRequest
	if err := json.NewDecoder(req.Body).Decode(&stagingRequest); err != nil {
		logger.Error("staging-request-body-decoding-failed", err)
		writeErrorResponse(logger, resp, http.StatusBadRequest, err)

		return
	}

	if err := s.stage(stagingGUID, stagingRequest); err != nil {
		reason := fmt.Sprintf("staging task with guid %s failed to start", stagingGUID)
		logger.Error("staging-failed", errors.Wrap(err, reason))
		writeErrorResponse(logger, resp, http.StatusInternalServerError, errors.New(reason))

		return
	}

	resp.WriteHeader(http.StatusAccepted)
}

func (s *Stage) stage(stagingGUID string, stagingRequest cf.StagingRequest) error {
	if stagingRequest.Lifecycle.DockerLifecycle != nil {
		return s.dockerStagingBifrost.TransferStaging(context.Background(), stagingGUID, stagingRequest)
	}

	return s.buildpackStagingBifrost.TransferStaging(context.Background(), stagingGUID, stagingRequest)
}

func (s *Stage) Complete(res http.ResponseWriter, req *http.Request, ps httprouter.Params) {
	stagingGUID := ps.ByName("staging_guid")
	logger := s.logger.Session("staging-complete", lager.Data{"staging-guid": stagingGUID})

	taskCompletedRequest := cf.StagingCompletedRequest{}

	err := json.NewDecoder(req.Body).Decode(&taskCompletedRequest)
	if err != nil {
		logger.Error("parsing-incoming-task-failed", err)
		res.WriteHeader(http.StatusBadRequest)

		return
	}

	if err = s.buildpackStagingBifrost.CompleteStaging(taskCompletedRequest); err != nil {
		res.WriteHeader(http.StatusInternalServerError)
		logger.Error("staging-completion-failed", err)

		return
	}

	logger.Info("posted-staging-complete")
}

func writeErrorResponse(logger lager.Logger, resp http.ResponseWriter, status int, err error) {
	resp.WriteHeader(status)
	encodingErr := json.NewEncoder(resp).Encode(&cf.Error{Message: err.Error()})

	if encodingErr != nil {
		logger.Error("failed-to-encode-error", encodingErr)
	}
}

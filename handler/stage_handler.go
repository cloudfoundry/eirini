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
	dockerStagingBifrost StagingBifrost
	logger               lager.Logger
}

func NewStageHandler(dockerStagingBifrost StagingBifrost, logger lager.Logger) *Stage {
	logger = logger.Session("staging-handler")

	return &Stage{
		dockerStagingBifrost: dockerStagingBifrost,
		logger:               logger,
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

	if stagingRequest.Lifecycle.DockerLifecycle == nil {
		err := errors.New("docker is the only supported lifecycle")
		logger.Error("staging-failed", err)
		writeErrorResponse(logger, resp, http.StatusBadRequest, err)

		return
	}

	if err := s.stage(stagingGUID, stagingRequest); err != nil {
		reason := fmt.Sprintf("failed to stage task with guid %q", stagingGUID)
		logger.Error("staging-failed", errors.Wrap(err, reason))
		writeErrorResponse(logger, resp, http.StatusInternalServerError, errors.Wrap(err, reason))

		return
	}

	resp.WriteHeader(http.StatusAccepted)
}

func (s *Stage) stage(stagingGUID string, stagingRequest cf.StagingRequest) error {
	return s.dockerStagingBifrost.TransferStaging(
		context.Background(),
		stagingGUID,
		stagingRequest,
	)
}

func writeErrorResponse(logger lager.Logger, resp http.ResponseWriter, status int, err error) {
	resp.WriteHeader(status)
	encodingErr := json.NewEncoder(resp).Encode(&cf.Error{Message: err.Error()})

	if encodingErr != nil {
		logger.Error("failed-to-encode-error", encodingErr)
	}
}

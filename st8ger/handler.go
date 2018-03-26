package st8ger

import (
	"bytes"
	"encoding/json"
	"io/ioutil"
	"net/http"

	"code.cloudfoundry.org/bbs/models"
	"code.cloudfoundry.org/lager"
	"code.cloudfoundry.org/runtimeschema/cc_messages"
	"github.com/julienschmidt/httprouter"
	"github.com/julz/cube"
)

func New(stager cube.St8ger, backend cube.Backend, logger lager.Logger) http.Handler {
	handler := httprouter.New()

	stagingHandler := NewStagingHandler(stager, backend, logger)

	handler.PUT("/v1/staging/:staging_guid", stagingHandler.Stage)
	handler.POST("/v1/staging/:staging_guid/completed", stagingHandler.StagingComplete)
	handler.DELETE("/v1/staging/:staging_guid", stagingHandler.StopStaging)

	return handler
}

type StagingHandler struct {
	stager  cube.St8ger
	backend cube.Backend
	logger  lager.Logger
}

func NewStagingHandler(stager cube.St8ger, backend cube.Backend, logger lager.Logger) *StagingHandler {
	logger = logger.Session("staging-handler")

	return &StagingHandler{
		stager:  stager,
		backend: backend,
		logger:  logger,
	}
}

func (handler *StagingHandler) Stage(resp http.ResponseWriter, req *http.Request, ps httprouter.Params) {
	stagingGuid := ps.ByName("staging_guid")
	logger := handler.logger.Session("staging-request", lager.Data{"staging-guid": stagingGuid})

	requestBody, err := ioutil.ReadAll(req.Body)
	if err != nil {
		logger.Error("read-body-failed", err)
		resp.WriteHeader(http.StatusInternalServerError)
		return
	}

	var stagingRequest cc_messages.StagingRequestFromCC
	err = json.Unmarshal(requestBody, &stagingRequest)
	if err != nil {
		logger.Error("unmarshal-request-failed", err)
		resp.WriteHeader(http.StatusBadRequest)
		return
	}

	envVars := []string{}
	for _, envVar := range stagingRequest.Environment {
		envVars = append(envVars, envVar.Name)
	}

	logger.Info("environment", lager.Data{"keys": envVars})

	stagingTask, err := handler.backend.CreateStagingTask(stagingGuid, stagingRequest)
	if err != nil {
		logger.Error("building-receipe-failed", err)
		resp.WriteHeader(http.StatusInternalServerError)
		return
	}

	err = handler.stager.Run(stagingTask)
	if err != nil {
		logger.Error("stage-app-failed", err)
		resp.WriteHeader(http.StatusInternalServerError)
		return
	}

	resp.WriteHeader(http.StatusAccepted)
}

func (handler *StagingHandler) StagingComplete(res http.ResponseWriter, req *http.Request, ps httprouter.Params) {
	stagingGuid := ps.ByName("staging_guid")
	logger := handler.logger.Session("staging-complete", lager.Data{"staging-guid": stagingGuid})

	task := &models.TaskCallbackResponse{}
	err := json.NewDecoder(req.Body).Decode(task)
	if err != nil {
		logger.Error("parsing-incoming-task-failed", err)
		res.WriteHeader(http.StatusBadRequest)
		return
	}

	var annotation cc_messages.StagingTaskAnnotation
	err = json.Unmarshal([]byte(task.Annotation), &annotation)
	if err != nil {
		res.WriteHeader(http.StatusBadRequest)
		logger.Error("parsing-annotation-failed", err)
		return
	}

	response, err := handler.backend.BuildStagingResponse(task)
	if err != nil {
		res.WriteHeader(http.StatusInternalServerError)
		logger.Error("error-creating-staging-response", err)
		return
	}

	responseJson, err := json.Marshal(response)
	if err != nil {
		res.WriteHeader(http.StatusBadRequest)
		logger.Error("get-staging-response-failed", err)
		return
	}

	request, err := http.NewRequest("POST", annotation.CompletionCallback, bytes.NewBuffer(responseJson))
	if err != nil {
		return
	}

	client := http.Client{}
	resp, err := client.Do(request)
	if err != nil {
		logger.Error("cc-staging-complete-failed", err)
		return
	}

	logger.Info("staging-complete-request-finished-with-status", lager.Data{"StatusCode": resp.StatusCode})
	logger.Info("posted-staging-complete")
}

func (handler *StagingHandler) StopStaging(resp http.ResponseWriter, req *http.Request, ps httprouter.Params) {
	//TODO
}

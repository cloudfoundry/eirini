package handlers

import (
	"net/http"

	"code.cloudfoundry.org/auctioneer"
	"code.cloudfoundry.org/bbs/db"
	"code.cloudfoundry.org/bbs/events"
	"code.cloudfoundry.org/bbs/models"
	"code.cloudfoundry.org/lager"
)

//go:generate counterfeiter -o fake_controllers/fake_actual_lrp_lifecycle_controller.go . ActualLRPLifecycleController
type ActualLRPLifecycleController interface {
	ClaimActualLRP(logger lager.Logger, processGuid string, index int32, actualLRPInstanceKey *models.ActualLRPInstanceKey) error
	StartActualLRP(logger lager.Logger, actualLRPKey *models.ActualLRPKey, actualLRPInstanceKey *models.ActualLRPInstanceKey, actualLRPNetInfo *models.ActualLRPNetInfo) error
	CrashActualLRP(logger lager.Logger, actualLRPKey *models.ActualLRPKey, actualLRPInstanceKey *models.ActualLRPInstanceKey, errorMessage string) error
	FailActualLRP(logger lager.Logger, key *models.ActualLRPKey, errorMessage string) error
	RemoveActualLRP(logger lager.Logger, processGuid string, index int32, instanceKey *models.ActualLRPInstanceKey) error
	RetireActualLRP(logger lager.Logger, key *models.ActualLRPKey) error
}

type ActualLRPLifecycleHandler struct {
	db               db.ActualLRPDB
	desiredLRPDB     db.DesiredLRPDB
	actualHub        events.Hub
	auctioneerClient auctioneer.Client
	controller       ActualLRPLifecycleController
	exitChan         chan<- struct{}
}

func NewActualLRPLifecycleHandler(
	controller ActualLRPLifecycleController,
	exitChan chan<- struct{},
) *ActualLRPLifecycleHandler {
	return &ActualLRPLifecycleHandler{
		controller: controller,
		exitChan:   exitChan,
	}
}

func (h *ActualLRPLifecycleHandler) ClaimActualLRP(logger lager.Logger, w http.ResponseWriter, req *http.Request) {
	var err error
	logger = logger.Session("claim-actual-lrp")

	request := &models.ClaimActualLRPRequest{}
	response := &models.ActualLRPLifecycleResponse{}
	defer func() { exitIfUnrecoverable(logger, h.exitChan, response.Error) }()
	defer writeResponse(w, response)

	err = parseRequest(logger, req, request)
	if err != nil {
		response.Error = models.ConvertError(err)
		return
	}

	err = h.controller.ClaimActualLRP(logger, request.ProcessGuid, request.Index, request.ActualLrpInstanceKey)
	response.Error = models.ConvertError(err)
}

func (h *ActualLRPLifecycleHandler) StartActualLRP(logger lager.Logger, w http.ResponseWriter, req *http.Request) {
	var err error

	logger = logger.Session("start-actual-lrp")

	request := &models.StartActualLRPRequest{}
	response := &models.ActualLRPLifecycleResponse{}

	defer func() { exitIfUnrecoverable(logger, h.exitChan, response.Error) }()
	defer writeResponse(w, response)

	err = parseRequest(logger, req, request)
	if err != nil {
		response.Error = models.ConvertError(err)
		return
	}

	err = h.controller.StartActualLRP(logger, request.ActualLrpKey, request.ActualLrpInstanceKey, request.ActualLrpNetInfo)
	response.Error = models.ConvertError(err)
}

func (h *ActualLRPLifecycleHandler) CrashActualLRP(logger lager.Logger, w http.ResponseWriter, req *http.Request) {
	logger = logger.Session("crash-actual-lrp")

	request := &models.CrashActualLRPRequest{}
	response := &models.ActualLRPLifecycleResponse{}
	defer func() { exitIfUnrecoverable(logger, h.exitChan, response.Error) }()
	defer writeResponse(w, response)

	err := parseRequest(logger, req, request)
	if err != nil {
		response.Error = models.ConvertError(err)
		return
	}

	actualLRPKey := request.ActualLrpKey
	actualLRPInstanceKey := request.ActualLrpInstanceKey

	err = h.controller.CrashActualLRP(logger, actualLRPKey, actualLRPInstanceKey, request.ErrorMessage)
	response.Error = models.ConvertError(err)
}

func (h *ActualLRPLifecycleHandler) FailActualLRP(logger lager.Logger, w http.ResponseWriter, req *http.Request) {
	var err error
	logger = logger.Session("fail-actual-lrp")

	request := &models.FailActualLRPRequest{}
	response := &models.ActualLRPLifecycleResponse{}

	defer func() { exitIfUnrecoverable(logger, h.exitChan, response.Error) }()
	defer writeResponse(w, response)

	err = parseRequest(logger, req, request)
	if err != nil {
		response.Error = models.ConvertError(err)
		return
	}

	err = h.controller.FailActualLRP(logger, request.ActualLrpKey, request.ErrorMessage)
	response.Error = models.ConvertError(err)
}

func (h *ActualLRPLifecycleHandler) RemoveActualLRP(logger lager.Logger, w http.ResponseWriter, req *http.Request) {
	var err error
	logger = logger.Session("remove-actual-lrp")

	request := &models.RemoveActualLRPRequest{}
	response := &models.ActualLRPLifecycleResponse{}

	defer func() { exitIfUnrecoverable(logger, h.exitChan, response.Error) }()
	defer writeResponse(w, response)

	err = parseRequest(logger, req, request)
	if err != nil {
		response.Error = models.ConvertError(err)
		return
	}

	err = h.controller.RemoveActualLRP(logger, request.ProcessGuid, request.Index, request.ActualLrpInstanceKey)
	response.Error = models.ConvertError(err)
}

func (h *ActualLRPLifecycleHandler) RetireActualLRP(logger lager.Logger, w http.ResponseWriter, req *http.Request) {
	logger = logger.Session("retire-actual-lrp")
	request := &models.RetireActualLRPRequest{}
	response := &models.ActualLRPLifecycleResponse{}

	var err error
	defer func() { exitIfUnrecoverable(logger, h.exitChan, response.Error) }()
	defer writeResponse(w, response)

	err = parseRequest(logger, req, request)
	if err != nil {
		response.Error = models.ConvertError(err)
		return
	}

	err = h.controller.RetireActualLRP(logger, request.ActualLrpKey)
	response.Error = models.ConvertError(err)
}

package handlers

import (
	"io/ioutil"
	"net/http"

	"code.cloudfoundry.org/bbs/format"
	"code.cloudfoundry.org/bbs/models"
	"code.cloudfoundry.org/lager"
)

func (h *DesiredLRPHandler) DesiredLRPs_r0(logger lager.Logger, w http.ResponseWriter, req *http.Request) {
	var err error
	logger = logger.Session("desired-lrps", lager.Data{"revision": 0})

	request := &models.DesiredLRPsRequest{}
	response := &models.DesiredLRPsResponse{}

	err = parseRequest(logger, req, request)
	if err == nil {
		var lrps []*models.DesiredLRP

		filter := models.DesiredLRPFilter{Domain: request.Domain}
		lrps, err = h.desiredLRPDB.DesiredLRPs(logger, filter)
		if err == nil {
			for i := range lrps {
				transformedLRP := lrps[i].VersionDownTo(format.V0)
				response.DesiredLrps = append(response.DesiredLrps, transformedLRP)
			}
		}
	}

	response.Error = models.ConvertError(err)

	writeResponse(w, response)
	exitIfUnrecoverable(logger, h.exitChan, response.Error)
}

func (h *DesiredLRPHandler) DesiredLRPs_r1(logger lager.Logger, w http.ResponseWriter, req *http.Request) {
	var err error
	logger = logger.Session("desired-lrps", lager.Data{"revision": 0})

	request := &models.DesiredLRPsRequest{}
	response := &models.DesiredLRPsResponse{}

	err = parseRequest(logger, req, request)
	if err == nil {
		var lrps []*models.DesiredLRP

		filter := models.DesiredLRPFilter{Domain: request.Domain}
		lrps, err = h.desiredLRPDB.DesiredLRPs(logger, filter)
		if err == nil {
			for i := range lrps {
				transformedLRP := lrps[i].VersionDownTo(format.V1)
				response.DesiredLrps = append(response.DesiredLrps, transformedLRP)
			}
		}
	}

	response.Error = models.ConvertError(err)

	writeResponse(w, response)
	exitIfUnrecoverable(logger, h.exitChan, response.Error)
}

func (h *DesiredLRPHandler) DesiredLRPByProcessGuid_r0(logger lager.Logger, w http.ResponseWriter, req *http.Request) {
	var err error
	logger = logger.Session("desired-lrp-by-process-guid", lager.Data{"revision": 0})

	request := &models.DesiredLRPByProcessGuidRequest{}
	response := &models.DesiredLRPResponse{}

	err = parseRequest(logger, req, request)
	if err == nil {
		var lrp *models.DesiredLRP
		lrp, err = h.desiredLRPDB.DesiredLRPByProcessGuid(logger, request.ProcessGuid)
		if err == nil {
			transformedLRP := lrp.VersionDownTo(format.V0)
			response.DesiredLrp = transformedLRP
		}
	}

	response.Error = models.ConvertError(err)

	writeResponse(w, response)
	exitIfUnrecoverable(logger, h.exitChan, response.Error)
}

func (h *DesiredLRPHandler) DesiredLRPByProcessGuid_r1(logger lager.Logger, w http.ResponseWriter, req *http.Request) {
	var err error
	logger = logger.Session("desired-lrp-by-process-guid", lager.Data{"revision": 1})

	request := &models.DesiredLRPByProcessGuidRequest{}
	response := &models.DesiredLRPResponse{}

	err = parseRequest(logger, req, request)
	if err == nil {
		var lrp *models.DesiredLRP
		lrp, err = h.desiredLRPDB.DesiredLRPByProcessGuid(logger, request.ProcessGuid)
		if err == nil {
			transformedLRP := lrp.VersionDownTo(format.V1)
			response.DesiredLrp = transformedLRP
		}
	}

	response.Error = models.ConvertError(err)

	writeResponse(w, response)
	exitIfUnrecoverable(logger, h.exitChan, response.Error)
}

func (h *DesiredLRPHandler) DesireDesiredLRP_r1(logger lager.Logger, w http.ResponseWriter, req *http.Request) {
	logger = logger.Session("desire-lrp", lager.Data{"revision": 1})

	request := &models.DesireLRPRequest{}
	response := &models.DesiredLRPLifecycleResponse{}
	defer func() { exitIfUnrecoverable(logger, h.exitChan, response.Error) }()
	defer writeResponse(w, response)

	err := parseRequestForDesireDesiredLRP_r1(logger, req, request)
	if err != nil {
		response.Error = models.ConvertError(err)
		return
	}

	err = h.desiredLRPDB.DesireLRP(logger, request.DesiredLrp)
	if err != nil {
		response.Error = models.ConvertError(err)
		return
	}

	desiredLRP, err := h.desiredLRPDB.DesiredLRPByProcessGuid(logger, request.DesiredLrp.ProcessGuid)
	if err != nil {
		response.Error = models.ConvertError(err)
		return
	}

	go h.desiredHub.Emit(models.NewDesiredLRPCreatedEvent(desiredLRP))

	schedulingInfo := request.DesiredLrp.DesiredLRPSchedulingInfo()
	h.startInstanceRange(logger, 0, schedulingInfo.Instances, &schedulingInfo)
}

func parseRequestForDesireDesiredLRP_r1(logger lager.Logger, req *http.Request, request *models.DesireLRPRequest) error {
	data, err := ioutil.ReadAll(req.Body)
	if err != nil {
		logger.Error("failed-to-read-body", err)
		return models.ErrUnknownError
	}

	err = request.Unmarshal(data)
	if err != nil {
		logger.Error("failed-to-parse-request-body", err)
		return models.ErrBadRequest
	}

	for i, mount := range request.DesiredLrp.VolumeMounts {
		request.DesiredLrp.VolumeMounts[i] = mount.VersionUpToV1()
	}

	if err := request.Validate(); err != nil {
		logger.Error("invalid-request", err)
		return models.NewError(models.Error_InvalidRequest, err.Error())
	}

	return nil
}

func (h *DesiredLRPHandler) DesireDesiredLRP_r0(logger lager.Logger, w http.ResponseWriter, req *http.Request) {
	logger = logger.Session("desire-lrp")

	request := &models.DesireLRPRequest{}
	response := &models.DesiredLRPLifecycleResponse{}
	defer func() { exitIfUnrecoverable(logger, h.exitChan, response.Error) }()
	defer writeResponse(w, response)

	err := parseRequestForDesireDesiredLRP_r0(logger, req, request)
	if err != nil {
		response.Error = models.ConvertError(err)
		return
	}

	err = h.desiredLRPDB.DesireLRP(logger, request.DesiredLrp)
	if err != nil {
		response.Error = models.ConvertError(err)
		return
	}

	desiredLRP, err := h.desiredLRPDB.DesiredLRPByProcessGuid(logger, request.DesiredLrp.ProcessGuid)
	if err != nil {
		response.Error = models.ConvertError(err)
		return
	}

	go h.desiredHub.Emit(models.NewDesiredLRPCreatedEvent(desiredLRP))

	schedulingInfo := request.DesiredLrp.DesiredLRPSchedulingInfo()
	h.startInstanceRange(logger, 0, schedulingInfo.Instances, &schedulingInfo)
}

func parseRequestForDesireDesiredLRP_r0(logger lager.Logger, req *http.Request, request *models.DesireLRPRequest) error {
	data, err := ioutil.ReadAll(req.Body)
	if err != nil {
		logger.Error("failed-to-read-body", err)
		return models.ErrUnknownError
	}

	err = request.Unmarshal(data)
	if err != nil {
		logger.Error("failed-to-parse-request-body", err)
		return models.ErrBadRequest
	}

	request.DesiredLrp.Action.SetTimeoutMsFromDeprecatedTimeoutNs()
	request.DesiredLrp.Setup.SetTimeoutMsFromDeprecatedTimeoutNs()
	request.DesiredLrp.Monitor.SetTimeoutMsFromDeprecatedTimeoutNs()
	request.DesiredLrp.StartTimeoutMs = int64(request.DesiredLrp.DeprecatedStartTimeoutS * 1000)

	if err := request.Validate(); err != nil {
		logger.Error("invalid-request", err)
		return models.NewError(models.Error_InvalidRequest, err.Error())
	}

	return nil
}

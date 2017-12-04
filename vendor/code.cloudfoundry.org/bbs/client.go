package bbs

import (
	"bytes"
	"crypto/tls"
	"errors"
	"fmt"
	"io/ioutil"
	"mime"
	"net/http"
	"net/url"
	"time"

	"code.cloudfoundry.org/bbs/events"
	"code.cloudfoundry.org/bbs/models"
	"code.cloudfoundry.org/cfhttp"
	"code.cloudfoundry.org/lager"
	"github.com/gogo/protobuf/proto"
	"github.com/tedsuo/rata"
	"github.com/vito/go-sse/sse"
)

const (
	ContentTypeHeader    = "Content-Type"
	XCfRouterErrorHeader = "X-Cf-Routererror"
	ProtoContentType     = "application/x-protobuf"
	KeepContainer        = true
	DeleteContainer      = false
)

//go:generate counterfeiter -o fake_bbs/fake_internal_client.go . InternalClient
//go:generate counterfeiter -o fake_bbs/fake_client.go . Client

/*
The InternalClient interface exposes all available endpoints of the BBS server,
including private endpoints which should be used exclusively by internal Diego
components. To interact with the BBS from outside of Diego, the Client
should be used instead.
*/
type InternalClient interface {
	Client

	ClaimActualLRP(logger lager.Logger, processGuid string, index int, instanceKey *models.ActualLRPInstanceKey) error
	StartActualLRP(logger lager.Logger, key *models.ActualLRPKey, instanceKey *models.ActualLRPInstanceKey, netInfo *models.ActualLRPNetInfo) error
	CrashActualLRP(logger lager.Logger, key *models.ActualLRPKey, instanceKey *models.ActualLRPInstanceKey, errorMessage string) error
	FailActualLRP(logger lager.Logger, key *models.ActualLRPKey, errorMessage string) error
	RemoveActualLRP(logger lager.Logger, processGuid string, index int, instanceKey *models.ActualLRPInstanceKey) error

	EvacuateClaimedActualLRP(lager.Logger, *models.ActualLRPKey, *models.ActualLRPInstanceKey) (bool, error)
	EvacuateRunningActualLRP(lager.Logger, *models.ActualLRPKey, *models.ActualLRPInstanceKey, *models.ActualLRPNetInfo, uint64) (bool, error)
	EvacuateStoppedActualLRP(lager.Logger, *models.ActualLRPKey, *models.ActualLRPInstanceKey) (bool, error)
	EvacuateCrashedActualLRP(lager.Logger, *models.ActualLRPKey, *models.ActualLRPInstanceKey, string) (bool, error)
	RemoveEvacuatingActualLRP(lager.Logger, *models.ActualLRPKey, *models.ActualLRPInstanceKey) error

	StartTask(logger lager.Logger, taskGuid string, cellID string) (bool, error)
	FailTask(logger lager.Logger, taskGuid, failureReason string) error
	CompleteTask(logger lager.Logger, taskGuid, cellId string, failed bool, failureReason, result string) error
}

/*
The External InternalClient can be used to access the BBS's public functionality.
It exposes methods for basic LRP and Task Lifecycles, Domain manipulation, and
event subscription.
*/
type Client interface {
	ExternalTaskClient
	ExternalDomainClient
	ExternalActualLRPClient
	ExternalDesiredLRPClient
	ExternalEventClient

	// Returns true if the BBS server is reachable
	Ping(logger lager.Logger) bool

	// Lists all Cells
	Cells(logger lager.Logger) ([]*models.CellPresence, error)
}

/*
The ExternalTaskClient is used to access Diego's ability to run one-off tasks.
More information about this API can be found in the bbs docs:

https://code.cloudfoundry.org/bbs/tree/master/doc/tasks.md
*/
type ExternalTaskClient interface {
	// Creates a Task from the given TaskDefinition
	DesireTask(logger lager.Logger, guid, domain string, def *models.TaskDefinition) error

	// Lists all Tasks
	Tasks(logger lager.Logger) ([]*models.Task, error)

	// List all Tasks that match filter
	TasksWithFilter(logger lager.Logger, filter models.TaskFilter) ([]*models.Task, error)

	// Lists all Tasks of the given domain
	TasksByDomain(logger lager.Logger, domain string) ([]*models.Task, error)

	// Lists all Tasks on the given cell
	TasksByCellID(logger lager.Logger, cellId string) ([]*models.Task, error)

	// Returns the Task with the given guid
	TaskByGuid(logger lager.Logger, guid string) (*models.Task, error)

	// Cancels the Task with the given task guid
	CancelTask(logger lager.Logger, taskGuid string) error

	// Resolves a Task with the given guid
	ResolvingTask(logger lager.Logger, taskGuid string) error

	// Deletes a completed task with the given guid
	DeleteTask(logger lager.Logger, taskGuid string) error
}

/*
The ExternalDomainClient is used to access and update Diego's domains.
*/
type ExternalDomainClient interface {
	// Lists the active domains
	Domains(logger lager.Logger) ([]string, error)

	// Creates a domain or bumps the ttl on an existing domain
	UpsertDomain(logger lager.Logger, domain string, ttl time.Duration) error
}

/*
The ExternalActualLRPClient is used to access and retire Actual LRPs
*/
type ExternalActualLRPClient interface {
	// Returns all ActualLRPGroups matching the given ActualLRPFilter
	ActualLRPGroups(lager.Logger, models.ActualLRPFilter) ([]*models.ActualLRPGroup, error)

	// Returns all ActualLRPGroups that have the given process guid
	ActualLRPGroupsByProcessGuid(logger lager.Logger, processGuid string) ([]*models.ActualLRPGroup, error)

	// Returns the ActualLRPGroup with the given process guid and instance index
	ActualLRPGroupByProcessGuidAndIndex(logger lager.Logger, processGuid string, index int) (*models.ActualLRPGroup, error)

	// Shuts down the ActualLRP matching the given ActualLRPKey, but does not modify the desired state
	RetireActualLRP(logger lager.Logger, key *models.ActualLRPKey) error
}

/*
The ExternalDesiredLRPClient is used to access and manipulate Disired LRPs.
*/
type ExternalDesiredLRPClient interface {
	// Lists all DesiredLRPs that match the given DesiredLRPFilter
	DesiredLRPs(lager.Logger, models.DesiredLRPFilter) ([]*models.DesiredLRP, error)

	// Returns the DesiredLRP with the given process guid
	DesiredLRPByProcessGuid(logger lager.Logger, processGuid string) (*models.DesiredLRP, error)

	// Returns all DesiredLRPSchedulingInfos that match the given DesiredLRPFilter
	DesiredLRPSchedulingInfos(lager.Logger, models.DesiredLRPFilter) ([]*models.DesiredLRPSchedulingInfo, error)

	// Creates the given DesiredLRP and its corresponding ActualLRPs
	DesireLRP(lager.Logger, *models.DesiredLRP) error

	// Updates the DesiredLRP matching the given process guid
	UpdateDesiredLRP(logger lager.Logger, processGuid string, update *models.DesiredLRPUpdate) error

	// Removes the DesiredLRP matching the given process guid
	RemoveDesiredLRP(logger lager.Logger, processGuid string) error
}

/*
The ExternalEventClient is used to subscribe to groups of Events.
*/
type ExternalEventClient interface {
	SubscribeToEvents(logger lager.Logger) (events.EventSource, error)
	SubscribeToTaskEvents(logger lager.Logger) (events.EventSource, error)
	SubscribeToEventsByCellID(logger lager.Logger, cellId string) (events.EventSource, error)
}

func newClient(url string) *client {
	return &client{
		httpClient:          cfhttp.NewClient(),
		streamingHTTPClient: cfhttp.NewStreamingClient(),
		reqGen:              rata.NewRequestGenerator(url, Routes),
	}
}

func NewClient(url string) InternalClient {
	return newClient(url)
}

func NewSecureClient(url, caFile, certFile, keyFile string, clientSessionCacheSize, maxIdleConnsPerHost int) (InternalClient, error) {
	return newSecureClient(url, caFile, certFile, keyFile, clientSessionCacheSize, maxIdleConnsPerHost, false)
}

func NewSecureSkipVerifyClient(url, certFile, keyFile string, clientSessionCacheSize, maxIdleConnsPerHost int) (InternalClient, error) {
	return newSecureClient(url, "", certFile, keyFile, clientSessionCacheSize, maxIdleConnsPerHost, true)
}

func newSecureClient(url, caFile, certFile, keyFile string, clientSessionCacheSize, maxIdleConnsPerHost int, skipVerify bool) (InternalClient, error) {
	client := newClient(url)

	tlsConfig, err := cfhttp.NewTLSConfig(certFile, keyFile, caFile)
	if err != nil {
		return nil, err
	}
	tlsConfig.ClientSessionCache = tls.NewLRUClientSessionCache(clientSessionCacheSize)

	tlsConfig.InsecureSkipVerify = skipVerify

	if tr, ok := client.httpClient.Transport.(*http.Transport); ok {
		tr.TLSClientConfig = tlsConfig
		tr.MaxIdleConnsPerHost = maxIdleConnsPerHost
	} else {
		return nil, errors.New("Invalid transport")
	}

	if tr, ok := client.streamingHTTPClient.Transport.(*http.Transport); ok {
		tr.TLSClientConfig = tlsConfig
		tr.MaxIdleConnsPerHost = maxIdleConnsPerHost
	} else {
		return nil, errors.New("Invalid transport")
	}

	return client, nil
}

type client struct {
	httpClient          *http.Client
	streamingHTTPClient *http.Client
	reqGen              *rata.RequestGenerator
}

func (c *client) Ping(logger lager.Logger) bool {
	response := models.PingResponse{}
	err := c.doRequest(logger, PingRoute, nil, nil, nil, &response)
	if err != nil {
		return false
	}
	return response.Available
}

func (c *client) Domains(logger lager.Logger) ([]string, error) {
	response := models.DomainsResponse{}
	err := c.doRequest(logger, DomainsRoute, nil, nil, nil, &response)
	if err != nil {
		return nil, err
	}
	return response.Domains, response.Error.ToError()
}

func (c *client) UpsertDomain(logger lager.Logger, domain string, ttl time.Duration) error {
	request := models.UpsertDomainRequest{
		Domain: domain,
		Ttl:    uint32(ttl.Seconds()),
	}
	response := models.UpsertDomainResponse{}
	err := c.doRequest(logger, UpsertDomainRoute, nil, nil, &request, &response)
	if err != nil {
		return err
	}
	return response.Error.ToError()
}

func (c *client) ActualLRPGroups(logger lager.Logger, filter models.ActualLRPFilter) ([]*models.ActualLRPGroup, error) {
	request := models.ActualLRPGroupsRequest{
		Domain: filter.Domain,
		CellId: filter.CellID,
	}
	response := models.ActualLRPGroupsResponse{}
	err := c.doRequest(logger, ActualLRPGroupsRoute, nil, nil, &request, &response)
	if err != nil {
		return nil, err
	}

	return response.ActualLrpGroups, response.Error.ToError()
}

func (c *client) ActualLRPGroupsByProcessGuid(logger lager.Logger, processGuid string) ([]*models.ActualLRPGroup, error) {
	request := models.ActualLRPGroupsByProcessGuidRequest{
		ProcessGuid: processGuid,
	}
	response := models.ActualLRPGroupsResponse{}
	err := c.doRequest(logger, ActualLRPGroupsByProcessGuidRoute, nil, nil, &request, &response)
	if err != nil {
		return nil, err
	}

	return response.ActualLrpGroups, response.Error.ToError()
}

func (c *client) ActualLRPGroupByProcessGuidAndIndex(logger lager.Logger, processGuid string, index int) (*models.ActualLRPGroup, error) {
	request := models.ActualLRPGroupByProcessGuidAndIndexRequest{
		ProcessGuid: processGuid,
		Index:       int32(index),
	}
	response := models.ActualLRPGroupResponse{}
	err := c.doRequest(logger, ActualLRPGroupByProcessGuidAndIndexRoute, nil, nil, &request, &response)
	if err != nil {
		return nil, err
	}

	return response.ActualLrpGroup, response.Error.ToError()
}

func (c *client) ClaimActualLRP(logger lager.Logger, processGuid string, index int, instanceKey *models.ActualLRPInstanceKey) error {
	request := models.ClaimActualLRPRequest{
		ProcessGuid:          processGuid,
		Index:                int32(index),
		ActualLrpInstanceKey: instanceKey,
	}
	response := models.ActualLRPLifecycleResponse{}
	err := c.doRequest(logger, ClaimActualLRPRoute, nil, nil, &request, &response)
	if err != nil {
		return err
	}
	return response.Error.ToError()
}

func (c *client) StartActualLRP(logger lager.Logger, key *models.ActualLRPKey, instanceKey *models.ActualLRPInstanceKey, netInfo *models.ActualLRPNetInfo) error {
	request := models.StartActualLRPRequest{
		ActualLrpKey:         key,
		ActualLrpInstanceKey: instanceKey,
		ActualLrpNetInfo:     netInfo,
	}
	response := models.ActualLRPLifecycleResponse{}
	err := c.doRequest(logger, StartActualLRPRoute, nil, nil, &request, &response)
	if err != nil {
		return err

	}
	return response.Error.ToError()
}

func (c *client) CrashActualLRP(logger lager.Logger, key *models.ActualLRPKey, instanceKey *models.ActualLRPInstanceKey, errorMessage string) error {
	request := models.CrashActualLRPRequest{
		ActualLrpKey:         key,
		ActualLrpInstanceKey: instanceKey,
		ErrorMessage:         errorMessage,
	}
	response := models.ActualLRPLifecycleResponse{}
	err := c.doRequest(logger, CrashActualLRPRoute, nil, nil, &request, &response)
	if err != nil {
		return err

	}
	return response.Error.ToError()
}

func (c *client) FailActualLRP(logger lager.Logger, key *models.ActualLRPKey, errorMessage string) error {
	request := models.FailActualLRPRequest{
		ActualLrpKey: key,
		ErrorMessage: errorMessage,
	}
	response := models.ActualLRPLifecycleResponse{}
	err := c.doRequest(logger, FailActualLRPRoute, nil, nil, &request, &response)
	if err != nil {
		return err

	}
	return response.Error.ToError()
}

func (c *client) RetireActualLRP(logger lager.Logger, key *models.ActualLRPKey) error {
	request := models.RetireActualLRPRequest{
		ActualLrpKey: key,
	}
	response := models.ActualLRPLifecycleResponse{}
	err := c.doRequest(logger, RetireActualLRPRoute, nil, nil, &request, &response)
	if err != nil {
		return err

	}
	return response.Error.ToError()
}

func (c *client) RemoveActualLRP(logger lager.Logger, processGuid string, index int, instanceKey *models.ActualLRPInstanceKey) error {
	request := models.RemoveActualLRPRequest{
		ProcessGuid:          processGuid,
		Index:                int32(index),
		ActualLrpInstanceKey: instanceKey,
	}

	response := models.ActualLRPLifecycleResponse{}
	err := c.doRequest(logger, RemoveActualLRPRoute, nil, nil, &request, &response)
	if err != nil {
		return err
	}
	return response.Error.ToError()
}

func (c *client) EvacuateClaimedActualLRP(logger lager.Logger, key *models.ActualLRPKey, instanceKey *models.ActualLRPInstanceKey) (bool, error) {
	return c.doEvacRequest(logger, EvacuateClaimedActualLRPRoute, KeepContainer, &models.EvacuateClaimedActualLRPRequest{
		ActualLrpKey:         key,
		ActualLrpInstanceKey: instanceKey,
	})
}

func (c *client) EvacuateCrashedActualLRP(logger lager.Logger, key *models.ActualLRPKey, instanceKey *models.ActualLRPInstanceKey, errorMessage string) (bool, error) {
	return c.doEvacRequest(logger, EvacuateCrashedActualLRPRoute, DeleteContainer, &models.EvacuateCrashedActualLRPRequest{
		ActualLrpKey:         key,
		ActualLrpInstanceKey: instanceKey,
		ErrorMessage:         errorMessage,
	})
}

func (c *client) EvacuateStoppedActualLRP(logger lager.Logger, key *models.ActualLRPKey, instanceKey *models.ActualLRPInstanceKey) (bool, error) {
	return c.doEvacRequest(logger, EvacuateStoppedActualLRPRoute, DeleteContainer, &models.EvacuateStoppedActualLRPRequest{
		ActualLrpKey:         key,
		ActualLrpInstanceKey: instanceKey,
	})
}

func (c *client) EvacuateRunningActualLRP(logger lager.Logger, key *models.ActualLRPKey, instanceKey *models.ActualLRPInstanceKey, netInfo *models.ActualLRPNetInfo, ttl uint64) (bool, error) {
	return c.doEvacRequest(logger, EvacuateRunningActualLRPRoute, KeepContainer, &models.EvacuateRunningActualLRPRequest{
		ActualLrpKey:         key,
		ActualLrpInstanceKey: instanceKey,
		ActualLrpNetInfo:     netInfo,
		Ttl:                  ttl,
	})
}

func (c *client) RemoveEvacuatingActualLRP(logger lager.Logger, key *models.ActualLRPKey, instanceKey *models.ActualLRPInstanceKey) error {
	request := models.RemoveEvacuatingActualLRPRequest{
		ActualLrpKey:         key,
		ActualLrpInstanceKey: instanceKey,
	}

	response := models.RemoveEvacuatingActualLRPResponse{}
	err := c.doRequest(logger, RemoveEvacuatingActualLRPRoute, nil, nil, &request, &response)
	if err != nil {
		return err
	}

	return response.Error.ToError()
}

func (c *client) DesiredLRPs(logger lager.Logger, filter models.DesiredLRPFilter) ([]*models.DesiredLRP, error) {
	request := models.DesiredLRPsRequest{
		Domain:       filter.Domain,
		ProcessGuids: filter.ProcessGuids,
	}
	response := models.DesiredLRPsResponse{}
	err := c.doRequest(logger, DesiredLRPsRoute, nil, nil, &request, &response)
	if err != nil {
		return nil, err
	}

	return response.DesiredLrps, response.Error.ToError()
}

func (c *client) DesiredLRPByProcessGuid(logger lager.Logger, processGuid string) (*models.DesiredLRP, error) {
	request := models.DesiredLRPByProcessGuidRequest{
		ProcessGuid: processGuid,
	}
	response := models.DesiredLRPResponse{}
	err := c.doRequest(logger, DesiredLRPByProcessGuidRoute, nil, nil, &request, &response)
	if err != nil {
		return nil, err
	}

	return response.DesiredLrp, response.Error.ToError()
}

func (c *client) DesiredLRPSchedulingInfos(logger lager.Logger, filter models.DesiredLRPFilter) ([]*models.DesiredLRPSchedulingInfo, error) {
	request := models.DesiredLRPsRequest{
		Domain:       filter.Domain,
		ProcessGuids: filter.ProcessGuids,
	}
	response := models.DesiredLRPSchedulingInfosResponse{}
	err := c.doRequest(logger, DesiredLRPSchedulingInfosRoute, nil, nil, &request, &response)
	if err != nil {
		return nil, err
	}

	return response.DesiredLrpSchedulingInfos, response.Error.ToError()
}

func (c *client) doDesiredLRPLifecycleRequest(logger lager.Logger, route string, request proto.Message) error {
	response := models.DesiredLRPLifecycleResponse{}
	err := c.doRequest(logger, route, nil, nil, request, &response)
	if err != nil {
		return err
	}
	return response.Error.ToError()
}

func (c *client) DesireLRP(logger lager.Logger, desiredLRP *models.DesiredLRP) error {
	request := models.DesireLRPRequest{
		DesiredLrp: desiredLRP,
	}
	return c.doDesiredLRPLifecycleRequest(logger, DesireDesiredLRPRoute, &request)
}

func (c *client) UpdateDesiredLRP(logger lager.Logger, processGuid string, update *models.DesiredLRPUpdate) error {
	request := models.UpdateDesiredLRPRequest{
		ProcessGuid: processGuid,
		Update:      update,
	}
	return c.doDesiredLRPLifecycleRequest(logger, UpdateDesiredLRPRoute, &request)
}

func (c *client) RemoveDesiredLRP(logger lager.Logger, processGuid string) error {
	request := models.RemoveDesiredLRPRequest{
		ProcessGuid: processGuid,
	}
	return c.doDesiredLRPLifecycleRequest(logger, RemoveDesiredLRPRoute, &request)
}

func (c *client) Tasks(logger lager.Logger) ([]*models.Task, error) {
	request := models.TasksRequest{}
	response := models.TasksResponse{}
	err := c.doRequest(logger, TasksRoute, nil, nil, &request, &response)
	if err != nil {
		return nil, err
	}

	return response.Tasks, response.Error.ToError()
}

func (c *client) TasksWithFilter(logger lager.Logger, filter models.TaskFilter) ([]*models.Task, error) {
	request := models.TasksRequest{
		Domain: filter.Domain,
		CellId: filter.CellID,
	}
	response := models.TasksResponse{}
	err := c.doRequest(logger, TasksRoute, nil, nil, &request, &response)
	if err != nil {
		return nil, err
	}
	return response.Tasks, response.Error.ToError()
}

func (c *client) TasksByDomain(logger lager.Logger, domain string) ([]*models.Task, error) {
	request := models.TasksRequest{
		Domain: domain,
	}
	response := models.TasksResponse{}
	err := c.doRequest(logger, TasksRoute, nil, nil, &request, &response)
	if err != nil {
		return nil, err
	}

	return response.Tasks, response.Error.ToError()
}

func (c *client) TasksByCellID(logger lager.Logger, cellId string) ([]*models.Task, error) {
	request := models.TasksRequest{
		CellId: cellId,
	}
	response := models.TasksResponse{}
	err := c.doRequest(logger, TasksRoute, nil, nil, &request, &response)
	if err != nil {
		return nil, err
	}

	return response.Tasks, response.Error.ToError()
}

func (c *client) TaskByGuid(logger lager.Logger, taskGuid string) (*models.Task, error) {
	request := models.TaskByGuidRequest{
		TaskGuid: taskGuid,
	}
	response := models.TaskResponse{}
	err := c.doRequest(logger, TaskByGuidRoute, nil, nil, &request, &response)
	if err != nil {
		return nil, err
	}

	return response.Task, response.Error.ToError()
}

func (c *client) doTaskLifecycleRequest(logger lager.Logger, route string, request proto.Message) error {
	response := models.TaskLifecycleResponse{}
	err := c.doRequest(logger, route, nil, nil, request, &response)
	if err != nil {
		return err
	}
	return response.Error.ToError()
}

func (c *client) DesireTask(logger lager.Logger, taskGuid, domain string, taskDef *models.TaskDefinition) error {
	route := DesireTaskRoute
	request := models.DesireTaskRequest{
		TaskGuid:       taskGuid,
		Domain:         domain,
		TaskDefinition: taskDef,
	}
	return c.doTaskLifecycleRequest(logger, route, &request)
}

func (c *client) StartTask(logger lager.Logger, taskGuid string, cellId string) (bool, error) {
	request := &models.StartTaskRequest{
		TaskGuid: taskGuid,
		CellId:   cellId,
	}
	response := &models.StartTaskResponse{}
	err := c.doRequest(logger, StartTaskRoute, nil, nil, request, response)
	if err != nil {
		return false, err
	}
	return response.ShouldStart, response.Error.ToError()
}

func (c *client) CancelTask(logger lager.Logger, taskGuid string) error {
	request := models.TaskGuidRequest{
		TaskGuid: taskGuid,
	}
	route := CancelTaskRoute
	return c.doTaskLifecycleRequest(logger, route, &request)
}

func (c *client) ResolvingTask(logger lager.Logger, taskGuid string) error {
	request := models.TaskGuidRequest{
		TaskGuid: taskGuid,
	}
	route := ResolvingTaskRoute
	return c.doTaskLifecycleRequest(logger, route, &request)
}

func (c *client) DeleteTask(logger lager.Logger, taskGuid string) error {
	request := models.TaskGuidRequest{
		TaskGuid: taskGuid,
	}
	route := DeleteTaskRoute
	return c.doTaskLifecycleRequest(logger, route, &request)
}

func (c *client) FailTask(logger lager.Logger, taskGuid, failureReason string) error {
	request := models.FailTaskRequest{
		TaskGuid:      taskGuid,
		FailureReason: failureReason,
	}
	route := FailTaskRoute
	return c.doTaskLifecycleRequest(logger, route, &request)
}

func (c *client) CompleteTask(logger lager.Logger, taskGuid, cellId string, failed bool, failureReason, result string) error {
	request := models.CompleteTaskRequest{
		TaskGuid:      taskGuid,
		CellId:        cellId,
		Failed:        failed,
		FailureReason: failureReason,
		Result:        result,
	}
	route := CompleteTaskRoute
	return c.doTaskLifecycleRequest(logger, route, &request)
}

func (c *client) subscribeToEvents(route string, cellId string) (events.EventSource, error) {
	request := models.EventsByCellId{
		CellId: cellId,
	}
	messageBody, err := proto.Marshal(&request)
	if err != nil {
		return nil, err
	}
	eventSource, err := sse.Connect(c.streamingHTTPClient, time.Second, func() *http.Request {
		request, err := c.reqGen.CreateRequest(route, nil, bytes.NewReader(messageBody))
		if err != nil {
			panic(err) // totally shouldn't happen
		}

		return request
	})

	if err != nil {
		return nil, err
	}

	return events.NewEventSource(eventSource), nil
}

func (c *client) SubscribeToEvents(logger lager.Logger) (events.EventSource, error) {
	return c.subscribeToEvents(EventStreamRoute_r0, "")
}

func (c *client) SubscribeToTaskEvents(logger lager.Logger) (events.EventSource, error) {
	return c.subscribeToEvents(TaskEventStreamRoute_r0, "")
}

func (c *client) SubscribeToEventsByCellID(logger lager.Logger, cellId string) (events.EventSource, error) {
	return c.subscribeToEvents(EventStreamRoute_r0, cellId)
}

func (c *client) Cells(logger lager.Logger) ([]*models.CellPresence, error) {
	response := models.CellsResponse{}
	err := c.doRequest(logger, CellsRoute, nil, nil, nil, &response)
	if err != nil {
		return nil, err
	}
	return response.Cells, response.Error.ToError()
}

func (c *client) createRequest(requestName string, params rata.Params, queryParams url.Values, message proto.Message) (*http.Request, error) {
	var messageBody []byte
	var err error
	if message != nil {
		messageBody, err = proto.Marshal(message)
		if err != nil {
			return nil, err
		}
	}

	request, err := c.reqGen.CreateRequest(requestName, params, bytes.NewReader(messageBody))
	if err != nil {
		return nil, err
	}

	request.URL.RawQuery = queryParams.Encode()
	request.ContentLength = int64(len(messageBody))
	request.Header.Set("Content-Type", ProtoContentType)
	return request, nil
}

func (c *client) doEvacRequest(logger lager.Logger, route string, defaultKeepContainer bool, request proto.Message) (bool, error) {
	var response models.EvacuationResponse
	err := c.doRequest(logger, route, nil, nil, request, &response)
	if err != nil {
		return defaultKeepContainer, err
	}

	return response.KeepContainer, response.Error.ToError()
}

func (c *client) doRequest(logger lager.Logger, requestName string, params rata.Params, queryParams url.Values, requestBody, responseBody proto.Message) error {
	logger = logger.Session("do-request")
	var err error
	var request *http.Request

	for attempts := 0; attempts < 3; attempts++ {
		logger.Debug("creating-request", lager.Data{"attempt": attempts + 1, "request_name": requestName})
		request, err = c.createRequest(requestName, params, queryParams, requestBody)
		if err != nil {
			logger.Error("failed-creating-request", err)
			return err
		}

		logger.Debug("doing-request", lager.Data{"attempt": attempts + 1, "request_path": request.URL.Path})

		start := time.Now().UnixNano()
		err = c.do(request, responseBody)
		finish := time.Now().UnixNano()

		if err != nil {
			logger.Error("failed-doing-request", err)
			time.Sleep(500 * time.Millisecond)
		} else {
			logger.Debug("complete", lager.Data{"request_path": request.URL.Path, "duration_in_ns": finish - start})
			break
		}
	}
	return err
}

func (c *client) do(request *http.Request, responseObject proto.Message) error {
	response, err := c.httpClient.Do(request)
	if err != nil {
		return err
	}
	defer func() {
		// don't worry about errors when closing the body
		_ = response.Body.Close()
	}()

	var parsedContentType string
	if contentType, ok := response.Header[ContentTypeHeader]; ok {
		parsedContentType, _, _ = mime.ParseMediaType(contentType[0])
	}

	if routerError, ok := response.Header[XCfRouterErrorHeader]; ok {
		return models.NewError(models.Error_RouterError, routerError[0])
	}

	if parsedContentType == ProtoContentType {
		return handleProtoResponse(response, responseObject)
	} else {
		return handleNonProtoResponse(response)
	}
}

func handleProtoResponse(response *http.Response, responseObject proto.Message) error {
	if responseObject == nil {
		return models.NewError(models.Error_InvalidRequest, "responseObject cannot be nil")
	}

	buf, err := ioutil.ReadAll(response.Body)
	if err != nil {
		return models.NewError(models.Error_InvalidResponse, fmt.Sprint("failed to read body: ", err.Error()))
	}

	err = proto.Unmarshal(buf, responseObject)
	if err != nil {
		return models.NewError(models.Error_InvalidProtobufMessage, fmt.Sprint("failed to unmarshal proto: ", err.Error()))
	}

	return nil
}

func handleNonProtoResponse(response *http.Response) error {
	if response.StatusCode > 299 {
		return models.NewError(models.Error_InvalidResponse, fmt.Sprintf("Invalid Response with status code: %d", response.StatusCode))
	}
	return nil
}

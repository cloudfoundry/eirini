package handlers

import (
	"net/http"

	"code.cloudfoundry.org/bbs/events"
	"code.cloudfoundry.org/bbs/models"
	"code.cloudfoundry.org/lager"
)

type EventController interface {
	Subscribe_r0(logger lager.Logger, w http.ResponseWriter, req *http.Request)
}

type EventHandler struct {
	desiredHub events.Hub
	actualHub  events.Hub
}

type TaskEventHandler struct {
	taskHub events.Hub
}

func NewEventHandler(desiredHub, actualHub events.Hub) *EventHandler {
	return &EventHandler{
		desiredHub: desiredHub,
		actualHub:  actualHub,
	}
}

func NewTaskEventHandler(taskHub events.Hub) *TaskEventHandler {
	return &TaskEventHandler{
		taskHub: taskHub,
	}
}

func streamEventsToResponse(logger lager.Logger, w http.ResponseWriter, eventChan <-chan models.Event, errorChan <-chan error) {
	w.Header().Add("Content-Type", "text/event-stream; charset=utf-8")
	w.Header().Add("Cache-Control", "no-cache, no-store, must-revalidate")
	w.Header().Add("Connection", "keep-alive")
	w.Header().Set("Transfer-Encoding", "identity")

	w.WriteHeader(http.StatusOK)

	conn, rw, err := w.(http.Hijacker).Hijack()
	if err != nil {
		return
	}

	defer func() {
		err := conn.Close()
		if err != nil {
			logger.Error("failed-to-close-connection", err)
		}
	}()

	if err := rw.Flush(); err != nil {
		logger.Error("failed-to-flush", err)
		return
	}

	var event models.Event
	eventID := 0
	closeNotifier := w.(http.CloseNotifier).CloseNotify()

	for {
		select {
		case event = <-eventChan:
		case err := <-errorChan:
			logger.Error("failed-to-get-next-event", err)
			return
		case <-closeNotifier:
			logger.Debug("received-close-notify")
			return
		}

		sseEvent, err := events.NewEventFromModelEvent(eventID, event)
		if err != nil {
			logger.Error("failed-to-marshal-event", err)
			return
		}

		err = sseEvent.Write(conn)
		if err != nil {
			logger.Error("failed-to-write-event", err)
			return
		}

		eventID++
	}
}

type EventFetcher func() (models.Event, error)

func streamSource(eventChan chan<- models.Event, errorChan chan<- error, closeChan chan struct{}, fetchEvent EventFetcher) {
	for {
		event, err := fetchEvent()
		if err != nil {
			select {
			case errorChan <- err:
			case <-closeChan:
			}
			return
		}
		select {
		case eventChan <- event:
		case <-closeChan:
			return
		}
	}
}

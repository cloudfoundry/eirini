package handlers

import "net/http"

type UnavailableHandler struct {
	handler          http.Handler
	serviceReadyChan <-chan struct{}
}

func NewUnavailableHandler(handler http.Handler, serviceReadyChan <-chan struct{}) *UnavailableHandler {
	u := &UnavailableHandler{
		handler:          handler,
		serviceReadyChan: serviceReadyChan,
	}

	return u
}

func (u *UnavailableHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	select {
	case <-u.serviceReadyChan:
		u.handler.ServeHTTP(w, r)
	default:
		w.WriteHeader(http.StatusServiceUnavailable)
	}
}

func UnavailableWrap(handler http.Handler, serviceReady <-chan struct{}) http.HandlerFunc {
	handler = NewUnavailableHandler(handler, serviceReady)

	return func(w http.ResponseWriter, r *http.Request) {
		handler.ServeHTTP(w, r)
	}
}

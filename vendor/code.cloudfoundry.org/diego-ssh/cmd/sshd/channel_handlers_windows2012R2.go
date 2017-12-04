// +build windows2012R2

package main

import (
	"code.cloudfoundry.org/diego-ssh/handlers"
)

func newChannelHandlers() map[string]handlers.NewChannelHandler {
	return map[string]handlers.NewChannelHandler{
		"session": handlers.NewSessionChannelHandler(),
	}
}

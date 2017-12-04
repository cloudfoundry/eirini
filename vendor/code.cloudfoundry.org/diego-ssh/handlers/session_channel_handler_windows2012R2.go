// +build windows2012R2

package handlers

import (
	"code.cloudfoundry.org/lager"
	"golang.org/x/crypto/ssh"
)

type SessionChannelHandler struct {
}

func NewSessionChannelHandler() *SessionChannelHandler {
	return &SessionChannelHandler{}
}

func (handler *SessionChannelHandler) HandleNewChannel(logger lager.Logger, newChannel ssh.NewChannel) {
	err := newChannel.Reject(ssh.Prohibited, "SSH is not supported on windows2012R2 cells")
	if err != nil {
		logger.Error("handle-new-session-channel-failed", err)
	}

	return
}

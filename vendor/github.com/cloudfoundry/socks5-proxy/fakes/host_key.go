package fakes

import "golang.org/x/crypto/ssh"

type HostKey struct {
	GetCall struct {
		CallCount int
		Receives  struct {
			Username   string
			PrivateKey string
			ServerURL  string
		}
		Returns struct {
			PublicKey ssh.PublicKey
			Error     error
		}
	}
}

func (h *HostKey) Get(username, privateKey, serverURL string) (ssh.PublicKey, error) {
	h.GetCall.CallCount++
	h.GetCall.Receives.Username = username
	h.GetCall.Receives.PrivateKey = privateKey
	h.GetCall.Receives.ServerURL = serverURL

	return h.GetCall.Returns.PublicKey, h.GetCall.Returns.Error
}

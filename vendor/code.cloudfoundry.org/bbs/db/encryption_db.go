package db

import "code.cloudfoundry.org/lager"

//go:generate counterfeiter . EncryptionDB

type EncryptionDB interface {
	EncryptionKeyLabel(logger lager.Logger) (string, error)
	SetEncryptionKeyLabel(logger lager.Logger, encryptionKeyLabel string) error
	PerformEncryption(logger lager.Logger) error
}

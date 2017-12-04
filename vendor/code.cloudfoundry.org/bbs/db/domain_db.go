package db

import "code.cloudfoundry.org/lager"

//go:generate counterfeiter . DomainDB
type DomainDB interface {
	Domains(logger lager.Logger) ([]string, error)
	UpsertDomain(lgger lager.Logger, domain string, ttl uint32) error
}

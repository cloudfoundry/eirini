package ldapshim

import (
	"crypto/tls"
	"time"

	"gopkg.in/ldap.v2"
)

//go:generate counterfeiter -o ldap_fake/fake_ldap.go . Ldap
type Ldap interface {
	Dial(network, addr string) (LdapConnection, error)
	DialTLS(network, addr string, config *tls.Config) (LdapConnection, error)
	NewSearchRequest(string, int, int, int, int, bool, string, []string, []ldap.Control) *ldap.SearchRequest
}

// Manually generated ldap interface
//go:generate counterfeiter -o ldap_fake/fake_ldap_connection.go . LdapConnection
type LdapConnection interface {
	SetTimeout(timeout time.Duration)
	Close()
	Bind(string, string) error
	Search(*ldap.SearchRequest) (*ldap.SearchResult, error)
}

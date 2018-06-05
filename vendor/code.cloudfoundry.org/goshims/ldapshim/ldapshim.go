package ldapshim

import (
	"crypto/tls"

	"gopkg.in/ldap.v2"
)

type LdapShim struct{}

func (l *LdapShim) Dial(network, addr string) (LdapConnection, error) {
	return ldap.Dial(network, addr)
}

func (l *LdapShim) DialTLS(network string, addr string, config *tls.Config) (LdapConnection, error) {
	return ldap.DialTLS(network, addr, config)
}

func (l *LdapShim) NewSearchRequest(a string, b int, c int, d int, e int, f bool, g string, h []string, i []ldap.Control) *ldap.SearchRequest {
	return ldap.NewSearchRequest(a, b, c, d, e, f, g, h, i)
}

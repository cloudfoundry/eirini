package ldapshim

import (
	"gopkg.in/ldap.v2"
)

type LdapShim struct{}

func (l *LdapShim) Dial(network, addr string) (LdapConnection, error) {
	return ldap.Dial(network, addr)
}

func (l *LdapShim) NewSearchRequest(a string, b int, c int, d int, e int, f bool, g string, h []string, i []ldap.Control) *ldap.SearchRequest {
	return ldap.NewSearchRequest(a, b, c, d, e, f, g, h, i)
}
package etcd

import (
	"path"

	"code.cloudfoundry.org/bbs/models"
	"code.cloudfoundry.org/lager"
)

func (db *ETCDDB) Domains(logger lager.Logger) ([]string, error) {
	response, err := db.client.Get(DomainSchemaRoot, false, true)
	if err != nil {
		if etcdErrCode(err) == ETCDErrKeyNotFound {
			return []string{}, nil
		}
		logger.Error("failed-to-fetch-domains", err)
		return nil, models.ErrUnknownError
	}

	domains := []string{}
	for _, child := range response.Node.Nodes {
		domains = append(domains, path.Base(child.Key))
	}

	return domains, nil
}

func (db *ETCDDB) UpsertDomain(logger lager.Logger, domain string, ttl uint32) error {
	_, err := db.client.Set(DomainSchemaPath(domain), []byte{}, uint64(ttl))
	if err != nil {
		logger.Error("failed-to-upsert-domain", err)
		return models.ErrUnknownError
	}
	return nil
}

func DomainSchemaPath(domain string) string {
	return path.Join(DomainSchemaRoot, domain)
}

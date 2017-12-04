package config

import (
	"errors"
	"fmt"
	"net/url"
	"strings"

	"code.cloudfoundry.org/bbs/db/etcd"
)

type ETCDConfig struct {
	ClusterUrls            []string `json:"etcd_cluster_urls"`
	CertFile               string   `json:"etcd_cert_file"`
	KeyFile                string   `json:"etcd_key_file"`
	CaFile                 string   `json:"etcd_ca_file"`
	ClientSessionCacheSize int      `json:"etcd_client_session_cache_size,omitempty"`
	MaxIdleConnsPerHost    int      `json:"etcd_max_idle_conns_per_host,omitempty"`
}

func DefaultETCDConfig() ETCDConfig {
	return ETCDConfig{
		MaxIdleConnsPerHost:    0,
		ClientSessionCacheSize: 0,
	}
}

func (config *ETCDConfig) Validate() (*etcd.ETCDOptions, error) {
	if len(config.ClusterUrls) == 0 && config.CaFile == "" && config.KeyFile == "" && config.CertFile == "" {
		return &etcd.ETCDOptions{IsConfigured: false}, nil
	}

	scheme := ""
	for _, uString := range config.ClusterUrls {
		u, err := url.Parse(uString)
		if err != nil {
			return nil, fmt.Errorf("Invalid cluster URL: '%s', error: [%s]", uString, err.Error())
		}
		if scheme == "" {
			if u.Scheme != "http" && u.Scheme != "https" {
				return nil, errors.New("Invalid scheme: " + uString)
			}
			scheme = u.Scheme
		} else if scheme != u.Scheme {
			return nil, fmt.Errorf("Multiple url schemes provided: %s", strings.Join(config.ClusterUrls, ","))
		}
	}

	isSSL := false
	if scheme == "https" {
		isSSL = true
		if config.CertFile == "" {
			return nil, errors.New("Cert file must be provided for https connections")
		}
		if config.KeyFile == "" {
			return nil, errors.New("Key file must be provided for https connections")
		}
	}

	return &etcd.ETCDOptions{
		CertFile:    config.CertFile,
		KeyFile:     config.KeyFile,
		CAFile:      config.CaFile,
		ClusterUrls: config.ClusterUrls,
		IsSSL:       isSSL,
		ClientSessionCacheSize: config.ClientSessionCacheSize,
		MaxIdleConnsPerHost:    config.MaxIdleConnsPerHost,
		IsConfigured:           true,
	}, nil
}

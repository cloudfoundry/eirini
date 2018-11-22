package cc_client

import (
	"bytes"
	"crypto/tls"
	"crypto/x509"
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"net"
	"net/http"
	"time"

	"code.cloudfoundry.org/lager"
	"code.cloudfoundry.org/runtimeschema/cc_messages"
	"code.cloudfoundry.org/urljoiner"
)

const (
	appCrashedPath           = "/internal/v4/apps/%s/crashed"
	appCrashedRequestTimeout = 5 * time.Second
)

//go:generate counterfeiter -o fakes/fake_cc_client.go . CcClient
type CcClient interface {
	AppCrashed(guid string, appCrashed cc_messages.AppCrashedRequest, logger lager.Logger) error
}

type ccClient struct {
	ccURI      string
	httpClient *http.Client
}

type BadResponseError struct {
	StatusCode int
}

func (b *BadResponseError) Error() string {
	return fmt.Sprintf("Crashed response POST failed with %d", b.StatusCode)
}

func NewTLSConfig(certFile string, keyFile string, caCertFile string) (*tls.Config, error) {
	tlsCert, err := tls.LoadX509KeyPair(certFile, keyFile)
	if err != nil {
		return nil, fmt.Errorf("failed to load keypair: %s", err.Error())
	}

	caCertBytes, err := ioutil.ReadFile(caCertFile)
	if err != nil {
		return nil, fmt.Errorf("failed to read ca cert file: %s", err.Error())
	}

	caCertPool := x509.NewCertPool()
	if ok := caCertPool.AppendCertsFromPEM(caCertBytes); !ok {
		return nil, errors.New("Unable to load ca cert")
	}

	tlsConfig := &tls.Config{
		Certificates:       []tls.Certificate{tlsCert},
		InsecureSkipVerify: false,
		ClientAuth:         tls.RequireAndVerifyClientCert,
		MinVersion:         tls.VersionTLS12,
		CipherSuites: []uint16{
			tls.TLS_ECDHE_RSA_WITH_AES_128_GCM_SHA256,
			tls.TLS_ECDHE_RSA_WITH_AES_256_GCM_SHA384,
		},
		RootCAs:   caCertPool,
		ClientCAs: caCertPool,
	}

	return tlsConfig, nil
}

func NewCcClient(baseURI string, tlsConfig *tls.Config) CcClient {
	httpClient := &http.Client{
		Timeout: appCrashedRequestTimeout,
		Transport: &http.Transport{
			Proxy: http.ProxyFromEnvironment,
			Dial: (&net.Dialer{
				Timeout:   10 * time.Second,
				KeepAlive: 30 * time.Second,
			}).Dial,
			TLSHandshakeTimeout: 10 * time.Second,
			TLSClientConfig:     tlsConfig,
		},
	}

	return &ccClient{
		ccURI:      urljoiner.Join(baseURI, appCrashedPath),
		httpClient: httpClient,
	}
}

func (cc *ccClient) AppCrashed(guid string, appCrashed cc_messages.AppCrashedRequest, logger lager.Logger) error {
	logger = logger.Session("cc-client")
	logger.Debug("delivering-app-crashed-response", lager.Data{"app_crashed": appCrashed})

	payload, err := json.Marshal(appCrashed)
	if err != nil {
		return err
	}

	request, err := http.NewRequest("POST", fmt.Sprintf(cc.ccURI, guid), bytes.NewReader(payload))
	if err != nil {
		return err
	}

	request.Header.Set("content-type", "application/json")

	response, err := cc.httpClient.Do(request)
	if err != nil {
		logger.Error("deliver-app-crashed-response-failed", err)
		return err
	}

	defer response.Body.Close()

	if response.StatusCode != http.StatusOK {
		return &BadResponseError{response.StatusCode}
	}

	logger.Debug("delivered-app-crashed-response")
	return nil
}

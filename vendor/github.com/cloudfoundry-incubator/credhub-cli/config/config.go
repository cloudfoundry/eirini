package config

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"
	"path"

	"github.com/cloudfoundry-incubator/credhub-cli/util"
)

const AuthClient = "credhub_cli"
const AuthPassword = ""

type Config struct {
	ApiURL             string
	AuthURL            string
	AccessToken        string
	RefreshToken       string
	InsecureSkipVerify bool
	CaCerts            []string
	ServerVersion      string
	ClientID           string
	ClientSecret       string
}

func ConfigDir() string {
	return path.Join(userHomeDir(), ".credhub")
}

func ConfigPath() string {
	return path.Join(ConfigDir(), "config.json")
}

func ReadConfig() Config {
	c := Config{}

	data, err := ioutil.ReadFile(ConfigPath())
	if err != nil {
		if !os.IsNotExist(err) {
			return c
		}
	}

	json.Unmarshal(data, &c)

	if server, ok := os.LookupEnv("CREDHUB_SERVER"); ok {
		c.ApiURL = util.AddDefaultSchemeIfNecessary(server)
		c.AuthURL = ""
		c.AccessToken = ""
		c.RefreshToken = ""
	}
	if client, ok := os.LookupEnv("CREDHUB_CLIENT"); ok {
		c.ClientID = client
	}
	if clientSecret, ok := os.LookupEnv("CREDHUB_SECRET"); ok {
		c.ClientSecret = clientSecret
	}
	if caCert, ok := os.LookupEnv("CREDHUB_CA_CERT"); ok {
		certs, err := ReadOrGetCaCerts([]string{caCert})
		if err != nil {
			fmt.Fprintf(os.Stderr, "error parsing CA certificates: %+v", err)
			return c
		}
		c.CaCerts = certs
	}

	return c
}

func WriteConfig(c Config) error {
	err := makeDirectory()
	if err != nil {
		return err
	}

	data, err := json.Marshal(c)
	if err != nil {
		return err
	}

	configPath := ConfigPath()
	return ioutil.WriteFile(configPath, data, 0600)
}

func RemoveConfig() error {
	return os.Remove(ConfigPath())
}

func (cfg *Config) UpdateTrustedCAs(caCerts []string) error {
	var certs []string

	for _, cert := range caCerts {
		certContents, err := util.ReadFileOrStringFromField(cert)
		if err != nil {
			return err
		}
		certs = append(certs, certContents)
	}

	cfg.CaCerts = certs

	return nil
}

func ReadOrGetCaCerts(caCerts []string) ([]string, error) {
	certs := []string{}

	for _, cert := range caCerts {
		certContents, err := util.ReadFileOrStringFromField(cert)
		if err != nil {
			return certs, err
		}
		certs = append(certs, certContents)
	}

	return certs, nil
}

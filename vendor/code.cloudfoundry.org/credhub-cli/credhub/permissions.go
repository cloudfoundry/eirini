package credhub

import (
	"io"
	"io/ioutil"
	"net/http"
	"net/url"

	"encoding/json"

	"code.cloudfoundry.org/credhub-cli/credhub/permissions"
	"github.com/hashicorp/go-version"
)

type permissionsResponse struct {
	CredentialName string                   `json:"credential_name"`
	Permissions    []permissions.V1_Permission `json:"permissions"`
}

func (ch *CredHub) GetPermissions(name string) ([]permissions.V1_Permission, error) {
	query := url.Values{}
	query.Set("credential_name", name)

	resp, err := ch.Request(http.MethodGet, "/api/v1/permissions", query, nil, true)
	if err != nil {
		return nil, err
	}

	defer resp.Body.Close()
	defer io.Copy(ioutil.Discard, resp.Body)
	var response permissionsResponse

	if err := json.NewDecoder(resp.Body).Decode(&response); err != nil {
		return nil, err
	}

	return response.Permissions, err
}

func (ch *CredHub) GetPermission(uuid string) (*permissions.Permission, error) {
	path := "/api/v2/permissions/" + uuid

	resp, err := ch.Request(http.MethodGet, path, nil, nil, true)

	if err != nil {
		return nil, err
	}

	defer resp.Body.Close()
	defer io.Copy(ioutil.Discard, resp.Body)
	var response permissions.Permission

	if err := json.NewDecoder(resp.Body).Decode(&response); err != nil {
		return nil, err
	}

	return &response, nil
}

func (ch *CredHub) addV1Permission(credName string, perms []permissions.V1_Permission) (*http.Response, error) {
	requestBody := map[string]interface{}{}
	requestBody["credential_name"] = credName
	requestBody["permissions"] = perms

	resp, err := ch.Request(http.MethodPost, "/api/v1/permissions", nil, requestBody, true)
	if err != nil {
		return nil, err
	}

	return resp, nil
}

func (ch *CredHub) addV2Permission(path string, actor string, ops []string) (*http.Response, error) {
	requestBody := map[string]interface{}{}
	requestBody["path"] = path
	requestBody["actor"] = actor
	requestBody["operations"] = ops

	resp, err := ch.Request(http.MethodPost, "/api/v2/permissions", nil, requestBody, true)
	if err != nil {
		return nil, err
	}

	return resp, nil
}

func (ch *CredHub) AddPermission(path string, actor string, ops []string) (*permissions.Permission, error) {
	serverVersion, err := ch.getServerVersion()
	if err != nil {
		return nil, err
	}

	var resp *http.Response
	isOlderVersion := serverVersion.Segments()[0] < 2

	if isOlderVersion {
		resp, err = ch.addV1Permission(path, []permissions.V1_Permission{{Actor: actor, Operations: ops}})
	} else {
		resp, err = ch.addV2Permission(path, actor, ops)
	}

	if err != nil {
		return nil, err
	}

	if isOlderVersion {
		return nil, nil
	}

	defer resp.Body.Close()
	defer io.Copy(ioutil.Discard, resp.Body)
	var response permissions.Permission

	if err := json.NewDecoder(resp.Body).Decode(&response); err != nil {
		return nil, err
	}

	return &response, nil
}

func (ch *CredHub) getServerVersion() (*version.Version, error) {
	if ch.cachedServerVersion == "" {
		serverVersion, err := ch.ServerVersion()
		if err != nil {
			return nil, err
		}
		ch.cachedServerVersion = serverVersion.String()
	}

	serverVersion, err := version.NewVersion(ch.cachedServerVersion)
	if err != nil {
		return nil, err
	}

	return serverVersion, nil
}

package credhub

import (
	"io"
	"io/ioutil"
	"net/http"

	"net/url"

	"encoding/json"

	"code.cloudfoundry.org/credhub-cli/credhub/permissions"
)

type permissionsResponse struct {
	CredentialName string                   `json:"credential_name"`
	Permissions    []permissions.Permission `json:"permissions"`
}

// GetPermissions returns the permissions of a credential.
func (ch *CredHub) GetPermissions(credName string) ([]permissions.Permission, error) {
	query := url.Values{}
	query.Set("credential_name", credName)

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

	return response.Permissions, nil
}

// AddPermissions adds permissions to a credential.
func (ch *CredHub) AddPermissions(credName string, perms []permissions.Permission) ([]permissions.Permission, error) {
	requestBody := map[string]interface{}{}
	requestBody["credential_name"] = credName
	requestBody["permissions"] = perms

	_, err := ch.Request(http.MethodPost, "/api/v1/permissions", nil, requestBody, true)
	if err != nil {
		return nil, err
	}

	return nil, nil
}

// DeletePermissions deletes permissions on a credential by actor.
func (ch *CredHub) DeletePermissions(credName string, actor string) error {
	panic("Not implemented")
}

package credhub

import (
	"encoding/json"
	"io"
	"io/ioutil"
	"net/http"

	"code.cloudfoundry.org/credhub-cli/credhub/credentials"
)

// Regenerate generates and returns a new credential version using the same parameters existing credential. The returned credential may be of any type.
func (ch *CredHub) Regenerate(name string) (credentials.Credential, error) {
	var cred credentials.Credential

	regenerateEndpoint := "/api/v1/data"

	requestBody := map[string]interface{}{}
	requestBody["name"] = name
	requestBody["regenerate"] = true

	resp, err := ch.Request(http.MethodPost, regenerateEndpoint, nil, requestBody, true)

	if err != nil {
		return credentials.Credential{}, err
	}

	defer resp.Body.Close()
	defer io.Copy(ioutil.Discard, resp.Body)
	dec := json.NewDecoder(resp.Body)
	err = dec.Decode(&cred)

	return cred, err
}

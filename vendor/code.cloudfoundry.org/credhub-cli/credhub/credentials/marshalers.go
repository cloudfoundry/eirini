package credentials

import (
	"encoding/json"

	"code.cloudfoundry.org/credhub-cli/errors"
)

func (c Credential) MarshalYAML() (interface{}, error) {
	return c.convertToMap()
}

func (c Credential) MarshalJSON() ([]byte, error) {
	result, err := c.convertToMap()
	if err != nil {
		return nil, err
	}
	return json.Marshal(result)
}

func (c Credential) convertToMap() (map[string]interface{}, error) {
	result := map[string]interface{}{
		"id":                 c.Id,
		"name":               c.Name,
		"version_created_at": c.VersionCreatedAt,
		"type":               c.Type,
	}

	_, ok := c.Value.(string)
	if ok {
		result["value"] = c.Value
	} else {
		value, ok := c.Value.(map[string]interface{})
		if !ok {
			return nil, errors.NewCatchAllError()
		}
		result["value"] = value
	}
	return result, nil
}

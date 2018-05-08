package models

import (
	"io/ioutil"

	"strconv"

	"regexp"

	"github.com/cloudfoundry-incubator/credhub-cli/errors"
	"gopkg.in/yaml.v2"
)

type CredentialBulkImport struct {
	Credentials []map[string]interface{} `yaml:"credentials"`
}

func (credentialBulkImport *CredentialBulkImport) ReadFile(filepath string) error {
	data, err := ioutil.ReadFile(filepath)
	if err != nil {
		return err
	}

	return credentialBulkImport.ReadBytes(data)
}

func (credentialBulkImport *CredentialBulkImport) ReadBytes(data []byte) error {
	if !hasCredentialTag(data) {
		return errors.NewNoCredentialsTag()
	}

	err := yaml.Unmarshal(data, credentialBulkImport)

	for i, credential := range credentialBulkImport.Credentials {
		credentialBulkImport.Credentials[i] = unpackCredential(credential)
	}

	if err != nil {
		return errors.NewInvalidImportYamlError()
	} else {
		return nil
	}
}

func unpackCredential(interfaceToInterfaceMap map[string]interface{}) map[string]interface{} {
	stringToInterfaceMap := make(map[string]interface{})
	stringToInterfaceMap["overwrite"] = true
	for key, value := range interfaceToInterfaceMap {
		stringToInterfaceMap[key] = unpackAnyType(value)
	}
	return stringToInterfaceMap
}

func unpackAnyType(value interface{}) interface{} {
	var unpackedValue interface{}
	switch typedValue := value.(type) {
	case map[interface{}]interface{}:
		unpackedValue = unpackMap(typedValue)
	case []interface{}:
		unpackedValue = unpackArray(typedValue)
	default:
		unpackedValue = value
	}
	return unpackedValue
}

func unpackKey(key interface{}) string {
	var unpackedKey string
	switch typedKey := key.(type) {
	case int:
		unpackedKey = strconv.Itoa(typedKey)
	case float32:
		unpackedKey = strconv.FormatFloat(float64(typedKey), 'f', -1, 32)
	case float64:
		unpackedKey = strconv.FormatFloat(typedKey, 'f', -1, 64)
	case bool:
		unpackedKey = strconv.FormatBool(typedKey)
	default:
		unpackedKey = key.(string)
	}
	return unpackedKey
}

func unpackMap(interfaceToInterfaceMap map[interface{}]interface{}) map[string]interface{} {
	stringToInterfaceMap := make(map[string]interface{})
	for key, value := range interfaceToInterfaceMap {
		unpackedKey := unpackKey(key)
		stringToInterfaceMap[unpackedKey] = unpackAnyType(value)
	}
	return stringToInterfaceMap
}

func unpackArray(array []interface{}) []interface{} {
	for i, value := range array {
		array[i] = unpackAnyType(value)
	}
	return array
}

func hasCredentialTag(data []byte) bool {
	hasCredentialTag, _ := regexp.Match("^(?:---[ \\n]+)?credentials:[^\\w]*", data)

	return hasCredentialTag
}

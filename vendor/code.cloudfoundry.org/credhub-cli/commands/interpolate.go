package commands

import (
	"fmt"
	"io/ioutil"
	"path"

	"code.cloudfoundry.org/credhub-cli/errors"
	"github.com/cloudfoundry/bosh-cli/director/template"
)

type InterpolateCommand struct {
	File   string `short:"f" long:"file"   description:"Path to the file to interpolate"`
	Prefix string `short:"p" long:"prefix" description:"Prefix to be applied to credential paths. Will not be applied to paths that start with '/'"`
	ClientCommand
}

func (c *InterpolateCommand) Execute([]string) error {
	if c.File == "" {
		return errors.NewMissingInterpolateParametersError()
	}

	fileContents, err := ioutil.ReadFile(c.File)
	if err != nil {
		return err
	}

	if len(fileContents) == 0 {
		return errors.NewEmptyTemplateError(c.File)
	}

	initialTemplate := template.NewTemplate(fileContents)

	credGetter := credentialGetter{
		clientCommand: c.ClientCommand,
		prefix:        c.Prefix,
	}

	renderedTemplate, err := initialTemplate.Evaluate(credGetter, nil, template.EvaluateOpts{ExpectAllKeys: true})
	if err != nil {
		return err
	}

	fmt.Println(string(renderedTemplate))
	return nil
}

type credentialGetter struct {
	clientCommand ClientCommand
	prefix        string
}

func (v credentialGetter) Get(varDef template.VariableDefinition) (interface{}, bool, error) {
	credName := varDef.Name
	if !path.IsAbs(varDef.Name) {
		credName = path.Join(v.prefix, credName)
	}

	credential, err := v.clientCommand.client.GetLatestVersion(credName)
	var result = credential.Value
	if mapString, ok := credential.Value.(map[string]interface{}); ok {
		mapInterface := map[interface{}]interface{}{}
		for k, v := range mapString {
			mapInterface[k] = v
		}
		result = mapInterface
	}
	return result, true, err
}

func (v credentialGetter) List() ([]template.VariableDefinition, error) {
	// not implemented
	return []template.VariableDefinition{}, nil
}

package inmemorygenerator

import (
	"code.cloudfoundry.org/quarks-utils/pkg/credsgen"
	"github.com/dchest/uniuri"
)

// GeneratePassword generates a random password
func (g InMemoryGenerator) GeneratePassword(name string, request credsgen.PasswordGenerationRequest) string {
	g.log.Debugf("Generating password %s", name)

	length := request.Length
	if length == 0 {
		length = credsgen.DefaultPasswordLength
	}

	return uniuri.NewLen(length)
}

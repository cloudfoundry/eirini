package format

import "github.com/gogo/protobuf/proto"

type Version byte

const (
	V0 Version = 0
	V1         = 1
	V2         = 2
)

var ValidVersions = []Version{V0, V1, V2}

//go:generate counterfeiter . Versioner
type Versioner interface {
	Version() Version
	Validate() error
}

//go:generate counterfeiter . ProtoVersioner
type ProtoVersioner interface {
	proto.Message
	Versioner
}

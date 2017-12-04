package format

import (
	"encoding/json"
	"errors"
	"fmt"
	"reflect"

	"code.cloudfoundry.org/lager"
	"github.com/gogo/protobuf/proto"
)

type EnvelopeFormat byte

const (
	LEGACY_JSON EnvelopeFormat = 0
	JSON        EnvelopeFormat = 1
	PROTO       EnvelopeFormat = 2
)

const EnvelopeOffset int = 2

func UnmarshalEnvelope(logger lager.Logger, unencodedPayload []byte, model Versioner) error {
	envelopeFormat, _ := EnvelopeMetadataFromPayload(unencodedPayload)

	var err error
	switch envelopeFormat {
	case LEGACY_JSON:
		err = UnmarshalJSON(logger, unencodedPayload, model)
	case JSON:
		err = UnmarshalJSON(logger, unencodedPayload[EnvelopeOffset:], model)
	case PROTO:
		protoModel, ok := model.(ProtoVersioner)
		if !ok {
			return errors.New("Model object incompatible with envelope format")
		}
		err = UnmarshalProto(logger, unencodedPayload[EnvelopeOffset:], protoModel)
	default:
		err = fmt.Errorf("unknown format %d", envelopeFormat)
		logger.Error("cannot-unmarshal-unknown-serialization-format", err)
	}

	return err
}

func MarshalEnvelope(format EnvelopeFormat, model Versioner) ([]byte, error) {
	var payload []byte
	var err error

	switch format {
	case PROTO:
		protoModel, ok := model.(ProtoVersioner)
		if !ok {
			return nil, errors.New("Model object incompatible with envelope format")
		}
		payload, err = MarshalProto(protoModel)
	case JSON:
		payload, err = MarshalJSON(model)
	case LEGACY_JSON:
		return MarshalJSON(model)
	default:
		err = fmt.Errorf("unknown format %d", format)
	}

	if err != nil {
		return nil, err
	}

	data := make([]byte, 0, len(payload)+EnvelopeOffset)
	data = append(data, byte(format), byte(model.Version()))
	data = append(data, payload...)

	return data, nil
}

func EnvelopeMetadataFromPayload(unencodedPayload []byte) (EnvelopeFormat, Version) {
	if !IsEnveloped(unencodedPayload) {
		return LEGACY_JSON, V0
	}
	return EnvelopeFormat(unencodedPayload[0]), Version(unencodedPayload[1])
}

func IsEnveloped(data []byte) bool {
	if len(data) < EnvelopeOffset {
		return false
	}

	switch EnvelopeFormat(data[0]) {
	case JSON, PROTO:
	default:
		return false
	}

	version := Version(data[1])
	for _, validVersion := range ValidVersions {
		if version == validVersion {
			return true
		}
	}

	return false
}

func UnmarshalJSON(logger lager.Logger, marshaledPayload []byte, model Versioner) error {
	err := json.Unmarshal(marshaledPayload, model)
	if err != nil {
		logger.Error("failed-to-json-unmarshal-payload", err)
		return err
	}
	return nil
}

func MarshalJSON(v Versioner) ([]byte, error) {
	bytes, err := json.Marshal(v)
	if err != nil {
		return nil, err
	}

	return bytes, nil
}

func UnmarshalProto(logger lager.Logger, marshaledPayload []byte, model ProtoVersioner) error {
	err := proto.Unmarshal(marshaledPayload, model)
	if err != nil {
		logger.Error("failed-to-proto-unmarshal-payload", err)
		return err
	}
	return nil
}

func MarshalProto(v ProtoVersioner) ([]byte, error) {
	bytes, err := proto.Marshal(v)
	if err != nil {
		return nil, err
	}

	return bytes, nil
}

func isNil(a interface{}) bool {
	if a == nil {
		return true
	}

	switch reflect.TypeOf(a).Kind() {
	case reflect.Chan, reflect.Func, reflect.Interface, reflect.Map, reflect.Ptr, reflect.Slice:
		return reflect.ValueOf(a).IsNil()
	}

	return false
}

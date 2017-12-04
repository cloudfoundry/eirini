package format

import (
	"code.cloudfoundry.org/bbs/encryption"
	"code.cloudfoundry.org/lager"
)

type Format struct {
	Encoding
	EnvelopeFormat
}

var (
	LEGACY_FORMATTING *Format = NewFormat(LEGACY_UNENCODED, LEGACY_JSON)
	FORMATTED_JSON    *Format = NewFormat(UNENCODED, JSON)
	ENCODED_PROTO     *Format = NewFormat(BASE64, PROTO)
	ENCRYPTED_PROTO   *Format = NewFormat(BASE64_ENCRYPTED, PROTO)
)

type serializer struct {
	encoder Encoder
}

type Serializer interface {
	Marshal(logger lager.Logger, format *Format, model Versioner) ([]byte, error)
	Unmarshal(logger lager.Logger, encodedPayload []byte, model Versioner) error
}

func NewSerializer(cryptor encryption.Cryptor) Serializer {
	return &serializer{
		encoder: NewEncoder(cryptor),
	}
}

func NewFormat(encoding Encoding, format EnvelopeFormat) *Format {
	return &Format{encoding, format}
}

func (s *serializer) Marshal(logger lager.Logger, format *Format, model Versioner) ([]byte, error) {
	envelopedPayload, err := MarshalEnvelope(format.EnvelopeFormat, model)
	if err != nil {
		return nil, err
	}

	return s.encoder.Encode(format.Encoding, envelopedPayload)
}

func (s *serializer) Unmarshal(logger lager.Logger, encodedPayload []byte, model Versioner) error {
	unencodedPayload, err := s.encoder.Decode(encodedPayload)
	if err != nil {
		return err
	}
	return UnmarshalEnvelope(logger, unencodedPayload, model)
}

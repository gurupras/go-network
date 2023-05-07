package network

import "io"

type Marshaler interface {
	Marshal(v interface{}) ([]byte, error)
}

type Unmarshaler interface {
	Unmarshal([]byte, interface{}) error
}

type Encoder interface {
	Encode(interface{}) error
}

type Decoder interface {
	Decode(interface{}) error
}

type CreateEncoder interface {
	CreateEncoder(io.Writer) Encoder
}

type CreateDecoder interface {
	CreateDecoder(io.Reader) Decoder
}

type Serializer interface {
	CreateEncoder
	Marshaler
}

type Deserializer interface {
	CreateDecoder
	Unmarshaler
}

type SerDe interface {
	Serializer
	Deserializer
}

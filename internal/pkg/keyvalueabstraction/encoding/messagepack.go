package encoding

import (
	"github.com/philippgille/gokv/encoding"
	"github.com/vmihailenco/msgpack/v5"
)

type Codec encoding.Codec

var MsgPack = &MsgPackEncoding{}

type MsgPackEncoding struct{}

func (m *MsgPackEncoding) Unmarshal(data []byte, v interface{}) error {
	return msgpack.Unmarshal(data, v)
}

func (m *MsgPackEncoding) Marshal(v interface{}) ([]byte, error) {
	return msgpack.Marshal(v)
}

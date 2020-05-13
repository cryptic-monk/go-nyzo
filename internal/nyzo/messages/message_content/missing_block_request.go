package message_content

import (
	"errors"
	"github.com/cryptic-monk/go-nyzo/internal/nyzo/messages/message_content/message_fields"
)

type MissingBlockRequest struct {
	Height int64
	Hash   []byte
}

func NewMissingBlockRequest(height int64, hash []byte) *MissingBlockRequest {
	return &MissingBlockRequest{height, hash}
}

// Serializable interface: data length when serialized
func (c *MissingBlockRequest) GetSerializedLength() int {
	return message_fields.SizeBlockHeight + message_fields.SizeHash
}

// Serializable interface: convert to bytes.
func (c *MissingBlockRequest) ToBytes() []byte {
	var serialized []byte
	serialized = append(serialized, message_fields.SerializeInt64(c.Height)...)
	serialized = append(serialized, c.Hash...)
	return serialized
}

// Serializable interface: convert from bytes.
func (c *MissingBlockRequest) FromBytes(bytes []byte) (int, error) {
	if len(bytes) < c.GetSerializedLength() {
		return 0, errors.New("invalid missing block request content")
	}
	position := 0
	c.Height = message_fields.DeserializeInt64(bytes[position : position+message_fields.SizeBlockHeight])
	position += message_fields.SizeBlockHeight
	c.Hash = bytes[position : position+message_fields.SizeHash]
	position += message_fields.SizeHash
	return position, nil
}

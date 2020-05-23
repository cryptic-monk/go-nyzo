package message_content

import (
	"github.com/cryptic-monk/go-nyzo/internal/nyzo/messages/message_content/message_fields"
	"io"
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
func (c *MissingBlockRequest) Read(r io.Reader) error {
	var err error
	c.Height, err = message_fields.ReadInt64(r)
	if err != nil {
		return err
	}
	c.Hash, err = message_fields.ReadHash(r)
	return err
}

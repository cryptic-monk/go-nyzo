package message_content

import (
	"github.com/cryptic-monk/go-nyzo/internal/nyzo/messages/message_content/message_fields"
	"io"
)

type MissingBlockVoteRequest struct {
	Height int64
}

func NewMissingBlockVoteRequest(height int64) *MissingBlockVoteRequest {
	return &MissingBlockVoteRequest{height}
}

// Serializable interface: data length when serialized
func (c *MissingBlockVoteRequest) GetSerializedLength() int {
	return message_fields.SizeBlockHeight
}

// Serializable interface: convert to bytes.
func (c *MissingBlockVoteRequest) ToBytes() []byte {
	var serialized []byte
	serialized = append(serialized, message_fields.SerializeInt64(c.Height)...)
	return serialized
}

// Serializable interface: convert from bytes.
func (c *MissingBlockVoteRequest) Read(r io.Reader) error {
	var err error
	c.Height, err = message_fields.ReadInt64(r)
	return err
}

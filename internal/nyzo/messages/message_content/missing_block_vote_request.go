package message_content

import (
	"errors"
	"github.com/cryptic-monk/go-nyzo/internal/nyzo/messages/message_content/message_fields"
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
func (c *MissingBlockVoteRequest) FromBytes(bytes []byte) (int, error) {
	if len(bytes) < c.GetSerializedLength() {
		return 0, errors.New("invalid missing block vote request content")
	}
	c.Height = message_fields.DeserializeInt64(bytes[0 : 0+message_fields.SizeBlockHeight])
	return message_fields.SizeBlockHeight, nil
}

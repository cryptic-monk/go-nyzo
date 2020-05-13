package message_content

import (
	"errors"
	"github.com/cryptic-monk/go-nyzo/internal/nyzo/messages/message_content/message_fields"
)

type BlockWithVotesRequest struct {
	Height int64
}

func NewBlockWithVotesRequest(height int64) *BlockWithVotesRequest {
	return &BlockWithVotesRequest{height}
}

// Serializable interface: data length when serialized
func (c *BlockWithVotesRequest) GetSerializedLength() int {
	return message_fields.SizeBlockHeight
}

// Serializable interface: convert to bytes.
func (c *BlockWithVotesRequest) ToBytes() []byte {
	var serialized []byte
	serialized = append(serialized, message_fields.SerializeInt64(c.Height)...)
	return serialized
}

// Serializable interface: convert from bytes.
func (c *BlockWithVotesRequest) FromBytes(bytes []byte) (int, error) {
	if len(bytes) < c.GetSerializedLength() {
		return 0, errors.New("invalid block with votes request content")
	}
	c.Height = message_fields.DeserializeInt64(bytes[0 : 0+message_fields.SizeBlockHeight])
	return message_fields.SizeBlockHeight, nil
}

package message_content

import (
	"github.com/cryptic-monk/go-nyzo/internal/nyzo/messages/message_content/message_fields"
	"io"
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
func (c *BlockWithVotesRequest) Read(r io.Reader) error {
	var err error
	c.Height, err = message_fields.ReadInt64(r)
	return err
}

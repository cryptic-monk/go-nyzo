package message_content

import (
	"github.com/cryptic-monk/go-nyzo/internal/nyzo/messages/message_content/message_fields"
	"io"
)

type BlockRequest struct {
	StartHeight        int64
	EndHeight          int64
	IncludeBalanceList bool
}

func NewBlockRequest(startHeight, endHeight int64, includeBalanceList bool) *BlockRequest {
	return &BlockRequest{startHeight, endHeight, includeBalanceList}
}

// Serializable interface: data length when serialized
func (c *BlockRequest) GetSerializedLength() int {
	return message_fields.SizeBlockHeight*2 + message_fields.SizeUnnamedByte
}

// Serializable interface: convert to bytes.
func (c *BlockRequest) ToBytes() []byte {
	var serialized []byte
	serialized = append(serialized, message_fields.SerializeInt64(c.StartHeight)...)
	serialized = append(serialized, message_fields.SerializeInt64(c.EndHeight)...)
	serialized = append(serialized, message_fields.SerializeBool(c.IncludeBalanceList)...)
	return serialized
}

// Serializable interface: convert from bytes.
func (c *BlockRequest) Read(r io.Reader) error {
	var err error
	c.StartHeight, err = message_fields.ReadInt64(r)
	if err != nil {
		return err
	}
	c.EndHeight, err = message_fields.ReadInt64(r)
	if err != nil {
		return err
	}
	c.IncludeBalanceList, err = message_fields.ReadBool(r)
	if err != nil {
		return err
	}
	return nil
}

package message_content

import (
	"errors"
	"github.com/cryptic-monk/go-nyzo/internal/nyzo/messages/message_content/message_fields"
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
func (c *BlockRequest) FromBytes(bytes []byte) (int, error) {
	if len(bytes) < c.GetSerializedLength() {
		return 0, errors.New("invalid block request content")
	}
	position := 0
	c.StartHeight = message_fields.DeserializeInt64(bytes[position : position+message_fields.SizeBlockHeight])
	position += message_fields.SizeBlockHeight
	c.EndHeight = message_fields.DeserializeInt64(bytes[position : position+message_fields.SizeBlockHeight])
	position += message_fields.SizeBlockHeight
	c.IncludeBalanceList = message_fields.DeserializeBool(bytes[position : position+message_fields.SizeUnnamedByte])
	position += message_fields.SizeUnnamedByte
	return position, nil
}

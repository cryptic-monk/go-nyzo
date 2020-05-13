package message_content

import (
	"errors"
	"github.com/cryptic-monk/go-nyzo/internal/nyzo/messages/message_content/message_fields"
)

type BootstrapResponse struct {
	FrozenEdgeHeight int64
	FrozenEdgeHash   []byte
	CycleVerifiers   [][]byte
}

func NewBootstrapRespone(frozenEdgeHeight int64, frozenEdgeHash []byte, cycleVerifiers [][]byte) *BootstrapResponse {
	return &BootstrapResponse{frozenEdgeHeight, frozenEdgeHash, cycleVerifiers}
}

// Serializable interface: data length when serialized
func (c *BootstrapResponse) GetSerializedLength() int {
	return message_fields.SizeBlockHeight + message_fields.SizeHash + message_fields.SizeUnnamedInt16 + (message_fields.SizeNodeIdentifier * len(c.CycleVerifiers))
}

// Serializable interface: convert to bytes.
func (c *BootstrapResponse) ToBytes() []byte {
	var serialized []byte
	serialized = append(serialized, message_fields.SerializeInt64(c.FrozenEdgeHeight)...)
	serialized = append(serialized, c.FrozenEdgeHash...)
	serialized = append(serialized, message_fields.SerializeInt16(int16(len(c.CycleVerifiers)))...)
	for _, id := range c.CycleVerifiers {
		serialized = append(serialized, id...)
	}
	return serialized
}

// Serializable interface: convert from bytes.
func (c *BootstrapResponse) FromBytes(bytes []byte) (int, error) {
	if len(bytes) < message_fields.SizeBlockHeight+message_fields.SizeHash+message_fields.SizeUnnamedInt16 {
		return 0, errors.New("invalid bootstrap request content 1")
	}
	position := 0
	c.FrozenEdgeHeight = message_fields.DeserializeInt64(bytes[position : position+message_fields.SizeBlockHeight])
	position += message_fields.SizeBlockHeight
	c.FrozenEdgeHash = bytes[position : position+message_fields.SizeHash]
	position += message_fields.SizeHash
	count := int(message_fields.DeserializeInt16(bytes[position : position+message_fields.SizeUnnamedInt16]))
	position += message_fields.SizeUnnamedInt16
	if len(bytes)-position < count*message_fields.SizeNodeIdentifier {
		return position, errors.New("invalid bootstrap request content 2")
	}
	c.CycleVerifiers = make([][]byte, 0, count)
	for i := 0; i < count; i++ {
		c.CycleVerifiers = append(c.CycleVerifiers, bytes[position:position+message_fields.SizeNodeIdentifier])
		position += message_fields.SizeNodeIdentifier
	}
	return position, nil
}

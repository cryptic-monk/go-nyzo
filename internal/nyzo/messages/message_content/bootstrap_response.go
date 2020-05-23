package message_content

import (
	"github.com/cryptic-monk/go-nyzo/internal/nyzo/messages/message_content/message_fields"
	"io"
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
func (c *BootstrapResponse) Read(r io.Reader) error {
	var err error
	c.FrozenEdgeHeight, err = message_fields.ReadInt64(r)
	if err != nil {
		return err
	}
	c.FrozenEdgeHash, err = message_fields.ReadHash(r)
	if err != nil {
		return err
	}
	count, err := message_fields.ReadInt16(r)
	if err != nil {
		return err
	}
	c.CycleVerifiers = make([][]byte, 0, count)
	for i := 0; i < int(count); i++ {
		id, err := message_fields.ReadNodeId(r)
		if err != nil {
			return err
		}
		c.CycleVerifiers = append(c.CycleVerifiers, id)
	}
	return nil
}

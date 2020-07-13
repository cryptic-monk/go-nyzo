package message_content

import (
	"github.com/cryptic-monk/go-nyzo/internal/nyzo/blockchain_data"
	"github.com/cryptic-monk/go-nyzo/internal/nyzo/messages/message_content/message_fields"
	"io"
)

type NewBlock struct {
	Block *blockchain_data.Block
}

func NewNewBlock(block *blockchain_data.Block) *NewBlock {
	return &NewBlock{block}
}

// Serializable interface: data length when serialized
func (c *NewBlock) GetSerializedLength() int {
	return c.Block.GetSerializedLength() + message_fields.SizePort
}

// Serializable interface: convert to bytes.
func (c *NewBlock) ToBytes() []byte {
	var serialized []byte
	serialized = append(serialized, c.Block.ToBytes()...)
	// legacy: port number (no longer used)
	serialized = append(serialized, message_fields.SerializeInt32(-1)...)
	return serialized
}

// Serializable interface: convert from bytes.
func (c *NewBlock) Read(r io.Reader) error {
	var err error
	c.Block = &blockchain_data.Block{}
	err = c.Block.Read(r)
	if err != nil {
		return err
	}
	// legacy: port number (no longer used)
	_, err = message_fields.ReadInt32(r)
	return err
}

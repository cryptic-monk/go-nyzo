package message_content

import (
	"errors"
	"fmt"
	"github.com/cryptic-monk/go-nyzo/internal/nyzo/blockchain_data"
	"github.com/cryptic-monk/go-nyzo/internal/nyzo/messages/message_content/message_fields"
	"io"
)

type BlockResponse struct {
	BalanceList *blockchain_data.BalanceList
	Blocks      []*blockchain_data.Block
}

func NewBlockResponse(balanceList *blockchain_data.BalanceList, blocks []*blockchain_data.Block) *BlockResponse {
	return &BlockResponse{balanceList, blocks}
}

// Serializable interface: data length when serialized
func (c *BlockResponse) GetSerializedLength() int {
	size := message_fields.SizeUnnamedByte
	if c.BalanceList != nil {
		size += c.BalanceList.GetSerializedLength()
	}
	size += message_fields.SizeFrozenBlockListLength
	for _, block := range c.Blocks {
		size += block.GetSerializedLength()
	}
	return size
}

// Serializable interface: convert to bytes.
func (c *BlockResponse) ToBytes() []byte {
	var serialized []byte
	serialized = append(serialized, message_fields.SerializeBool(c.BalanceList != nil)...)
	if c.BalanceList != nil {
		serialized = append(serialized, c.BalanceList.ToBytes()...)
	}
	serialized = append(serialized, message_fields.SerializeInt16(int16(len(c.Blocks)))...)
	for _, block := range c.Blocks {
		serialized = append(serialized, block.ToBytes()...)
	}
	return serialized
}

// Serializable interface: convert from bytes.
func (c *BlockResponse) Read(r io.Reader) error {
	hasBalanceList, err := message_fields.ReadBool(r)
	if err != nil {
		return err
	}
	if hasBalanceList {
		c.BalanceList, err = blockchain_data.ReadNewBalanceList(r)
		if err != nil {
			return err
		}
	}
	blockCount, err := message_fields.ReadInt16(r)
	if err != nil {
		return err
	}
	if blockCount < 0 {
		return errors.New(fmt.Sprintf("invalid block count in block response: %d", blockCount))
	}
	if blockCount > 10 {
		blockCount = 10
	}
	c.Blocks = make([]*blockchain_data.Block, 0, blockCount)
	for i := 0; i < int(blockCount); i++ {
		block, err := blockchain_data.ReadNewBlock(r)
		if err != nil {
			return err
		}
		c.Blocks = append(c.Blocks, block)
	}
	return nil
}

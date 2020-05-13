package message_content

import (
	"errors"
	"fmt"
	"github.com/cryptic-monk/go-nyzo/internal/nyzo/blockchain_data"
	"github.com/cryptic-monk/go-nyzo/internal/nyzo/messages/message_content/message_fields"
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
func (c *BlockResponse) FromBytes(bytes []byte) (int, error) {
	if len(bytes) < message_fields.SizeBool+message_fields.SizeFrozenBlockListLength {
		return 0, errors.New("invalid block response content 1")
	}
	var bytesConsumed int
	position := message_fields.SizeBool
	if bytes[0] == 1 {
		c.BalanceList, bytesConsumed = blockchain_data.NewBalanceListFromBytes(bytes[position:])
		if c.BalanceList == nil {
			return 0, errors.New("invalid block response content 2")
		} else {
			position += bytesConsumed
		}
	}
	if len(bytes)-position < message_fields.SizeFrozenBlockListLength {
		return 0, errors.New("invalid block response content 3")
	}
	blockCount := message_fields.DeserializeInt16(bytes[position : position+message_fields.SizeFrozenBlockListLength])
	position += message_fields.SizeFrozenBlockListLength
	if blockCount < 0 {
		return 0, errors.New(fmt.Sprintf("invalid block count in block response: %d", blockCount))
	}
	if blockCount > 10 {
		blockCount = 10
	}
	c.Blocks = make([]*blockchain_data.Block, 0, blockCount)
	for i := 0; i < int(blockCount); i++ {
		block, bytesConsumed := blockchain_data.NewBlockFromBytes(bytes[position:])
		if block == nil {
			return 0, errors.New("invalid block response content 5")
		} else {
			c.Blocks = append(c.Blocks, block)
			position += bytesConsumed
		}
	}
	return position, nil
}

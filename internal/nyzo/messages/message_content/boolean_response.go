package message_content

import (
	"errors"
	"github.com/cryptic-monk/go-nyzo/internal/nyzo/messages/message_content/message_fields"
	"math"
)

type BooleanResponse struct {
	Success bool
	Message string
}

func NewBooleanResponse(success bool, message string) *BooleanResponse {
	return &BooleanResponse{success, message}
}

// Serializable interface: data length when serialized
func (c *BooleanResponse) GetSerializedLength() int {
	return message_fields.SizeBool + len(message_fields.SerializeString(c.Message, math.MaxInt32))
}

// Serializable interface: convert to bytes.
func (c *BooleanResponse) ToBytes() []byte {
	var serialized []byte
	serialized = append(serialized, message_fields.SerializeBool(c.Success)...)
	serialized = append(serialized, message_fields.SerializeString(c.Message, math.MaxInt32)...)
	return serialized
}

// Serializable interface: convert from bytes.
func (c *BooleanResponse) FromBytes(bytes []byte) (int, error) {
	if len(bytes) < message_fields.SizeBool+message_fields.SizeStringLength {
		return 0, errors.New("invalid boolean response content")
	}
	position := 0
	c.Success = message_fields.DeserializeBool(bytes[position : position+message_fields.SizeBool])
	position += message_fields.SizeBool
	var consumed int
	c.Message, consumed = message_fields.DeserializeString(bytes[position:])
	position += consumed
	return consumed, nil
}

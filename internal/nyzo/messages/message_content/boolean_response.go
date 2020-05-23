package message_content

import (
	"github.com/cryptic-monk/go-nyzo/internal/nyzo/messages/message_content/message_fields"
	"io"
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
func (c *BooleanResponse) Read(r io.Reader) error {
	var err error
	c.Success, err = message_fields.ReadBool(r)
	if err != nil {
		return err
	}
	c.Message, err = message_fields.ReadString(r)
	return err
}

package message_content

import (
	"errors"
	"github.com/cryptic-monk/go-nyzo/internal/nyzo/messages/message_content/message_fields"
)

type StatusResponse struct {
	Lines []string
}

func NewStatusResponse(lines []string) *StatusResponse {
	return &StatusResponse{lines}
}

// Serializable interface: data length when serialized
func (c *StatusResponse) GetSerializedLength() int {
	size := message_fields.SizeUnnamedByte
	for _, s := range c.Lines {
		size += message_fields.SerializedStringLength(s, 1024)
	}
	return size
}

// Serializable interface: convert to bytes.
func (c *StatusResponse) ToBytes() []byte {
	var serialized []byte
	serialized = append(serialized, byte(len(c.Lines)))
	for _, s := range c.Lines {
		serialized = append(serialized, message_fields.SerializeString(s, 1024)...)
	}
	return serialized
}

// Serializable interface: convert from bytes.
func (c *StatusResponse) FromBytes(bytes []byte) (int, error) {
	if len(bytes) < message_fields.SizeUnnamedByte {
		return 0, errors.New("invalid status response content")
	}
	position := 0
	count := int(bytes[0])
	position += message_fields.SizeUnnamedByte
	c.Lines = make([]string, 0, count)
	for i := 0; i < count; i++ {
		s, bytesConsumed := message_fields.DeserializeString(bytes[position:])
		position += bytesConsumed
		c.Lines = append(c.Lines, s)
	}
	return position, nil
}

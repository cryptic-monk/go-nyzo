package message_content

import (
	"github.com/cryptic-monk/go-nyzo/internal/nyzo/messages/message_content/message_fields"
	"io"
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
func (c *StatusResponse) Read(r io.Reader) error {
	count, err := message_fields.ReadByte(r)
	if err != nil {
		return err
	}
	c.Lines = make([]string, 0, count)
	for i := 0; i < int(count); i++ {
		s, err := message_fields.ReadString(r)
		if err != nil {
			return err
		}
		c.Lines = append(c.Lines, s)
	}
	return nil
}

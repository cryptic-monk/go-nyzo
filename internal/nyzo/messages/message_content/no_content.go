package message_content

import (
	"io"
)

// An empty message.
type NoContent struct {
}

// Serializable interface: data length when serialized.
func (c *NoContent) GetSerializedLength() int {
	return 0
}

// Serializable interface: to data bytes.
func (c *NoContent) ToBytes() []byte {
	return make([]byte, 0)
}

// Serializable interface: from data bytes.
func (c *NoContent) Read(r io.Reader) error {
	return nil
}

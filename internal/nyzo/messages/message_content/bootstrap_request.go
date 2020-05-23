package message_content

import (
	"github.com/cryptic-monk/go-nyzo/internal/nyzo/messages/message_content/message_fields"
	"io"
)

type BootstrapRequest struct {
	Port int32
}

func NewBootstrapRequest(port int32) *BootstrapRequest {
	return &BootstrapRequest{port}
}

// Serializable interface: data length when serialized
func (c *BootstrapRequest) GetSerializedLength() int {
	return message_fields.SizePort
}

// Serializable interface: convert to bytes.
func (c *BootstrapRequest) ToBytes() []byte {
	var serialized []byte
	serialized = append(serialized, message_fields.SerializeInt32(c.Port)...)
	return serialized
}

// Serializable interface: convert from bytes.
func (c *BootstrapRequest) Read(r io.Reader) error {
	var err error
	c.Port, err = message_fields.ReadInt32(r)
	return err
}

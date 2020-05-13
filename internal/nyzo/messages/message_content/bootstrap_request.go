package message_content

import (
	"errors"
	"github.com/cryptic-monk/go-nyzo/internal/nyzo/messages/message_content/message_fields"
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
func (c *BootstrapRequest) FromBytes(bytes []byte) (int, error) {
	if len(bytes) < c.GetSerializedLength() {
		return 0, errors.New("invalid bootstrap request content")
	}
	c.Port = message_fields.DeserializeInt32(bytes[0 : 0+message_fields.SizePort])
	return message_fields.SizePort, nil
}

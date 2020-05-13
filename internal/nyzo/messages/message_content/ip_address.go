package message_content

import (
	"errors"
	"github.com/cryptic-monk/go-nyzo/internal/nyzo/messages/message_content/message_fields"
)

type IpAddress struct {
	Address []byte
}

func NewIpAddress(address []byte) *IpAddress {
	return &IpAddress{address}
}

// Serializable interface: data length when serialized
func (c *IpAddress) GetSerializedLength() int {
	return message_fields.SizeIPAddress
}

// Serializable interface: convert to bytes.
func (c *IpAddress) ToBytes() []byte {
	return c.Address
}

// Serializable interface: convert from bytes.
func (c *IpAddress) FromBytes(bytes []byte) (int, error) {
	if len(bytes) != message_fields.SizeIPAddress {
		return 0, errors.New("invalid ip address message content")
	}
	c.Address = bytes
	return message_fields.SizeIPAddress, nil
}

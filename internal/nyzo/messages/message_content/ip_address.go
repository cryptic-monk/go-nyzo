package message_content

import (
	"github.com/cryptic-monk/go-nyzo/internal/nyzo/messages/message_content/message_fields"
	"io"
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
func (c *IpAddress) Read(r io.Reader) error {
	var err error
	c.Address, err = message_fields.ReadBytes(r, message_fields.SizeIPAddress)
	return err
}

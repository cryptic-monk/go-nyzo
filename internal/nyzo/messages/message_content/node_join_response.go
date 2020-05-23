package message_content

import (
	"github.com/cryptic-monk/go-nyzo/internal/nyzo/messages/message_content/message_fields"
	"io"
)

type NodeJoinResponse struct {
	Nickname string
	PortTcp  int32
	PortUdp  int32
}

func NewNodeJoinResponse(nickname string, portTcp, portUdp int32) *NodeJoinResponse {
	return &NodeJoinResponse{nickname, portTcp, portUdp}
}

// Serializable interface: data length when serialized
func (c *NodeJoinResponse) GetSerializedLength() int {
	return message_fields.SerializedStringLength(c.Nickname, maximumNicknameLengthBytes) + message_fields.SizePort*2
}

// Serializable interface: convert to bytes.
func (c *NodeJoinResponse) ToBytes() []byte {
	var serialized []byte
	serialized = append(serialized, message_fields.SerializeString(c.Nickname, maximumNicknameLengthBytes)...)
	serialized = append(serialized, message_fields.SerializeInt32(c.PortTcp)...)
	serialized = append(serialized, message_fields.SerializeInt32(c.PortUdp)...)
	return serialized
}

// Serializable interface: convert from bytes.
func (c *NodeJoinResponse) Read(r io.Reader) error {
	var err error
	c.Nickname, err = message_fields.ReadString(r)
	if err != nil {
		return err
	}
	c.PortTcp, err = message_fields.ReadInt32(r)
	if err != nil {
		return err
	}
	c.PortUdp, err = message_fields.ReadInt32(r)
	return err
}

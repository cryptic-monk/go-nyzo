/*
TODO: the Java version sends V1 join messages during the startup process, but V2s in some other contexts. Clarify whether this is really needed, or whether we can just live with V2 everywhere.
*/
package message_content

import (
	"github.com/cryptic-monk/go-nyzo/internal/nyzo/messages/message_content/message_fields"
	"io"
)

const (
	maximumNicknameLengthBytes = 50 // max length of utf8-encoded Nickname bytes that we'll transfer
)

type NodeJoin struct {
	PortTcp  int32
	PortUdp  int32
	Nickname string
}

func NewNodeJoin(portTcp, portUdp int32, nickname string) *NodeJoin {
	return &NodeJoin{portTcp, portUdp, nickname}
}

// Serializable interface: data length when serialized
func (c *NodeJoin) GetSerializedLength() int {
	return message_fields.SizePort*2 + message_fields.SerializedStringLength(c.Nickname, maximumNicknameLengthBytes)
}

// Serializable interface: convert to bytes.
func (c *NodeJoin) ToBytes() []byte {
	var serialized []byte
	serialized = append(serialized, message_fields.SerializeInt32(c.PortTcp)...)
	serialized = append(serialized, message_fields.SerializeInt32(c.PortUdp)...)
	serialized = append(serialized, message_fields.SerializeString(c.Nickname, maximumNicknameLengthBytes)...)
	return serialized
}

// Serializable interface: convert from bytes.
func (c *NodeJoin) Read(r io.Reader) error {
	var err error
	c.PortTcp, err = message_fields.ReadInt32(r)
	if err != nil {
		return err
	}
	c.PortUdp, err = message_fields.ReadInt32(r)
	if err != nil {
		return err
	}
	c.Nickname, err = message_fields.ReadString(r)
	return err
}

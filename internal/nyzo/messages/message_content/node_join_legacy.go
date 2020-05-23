/*
TODO: the Java version sends V1 join messages during the startup process, but V2s in some other contexts. Clarify whether this is really needed, or whether we can just live with V2 everywhere.
*/
package message_content

import (
	"github.com/cryptic-monk/go-nyzo/internal/nyzo/messages/message_content/message_fields"
	"io"
)

type NodeJoinLegacy struct {
	Port     int32
	Nickname string
}

func NewNodeJoinLegacy(port int32, nickname string) *NodeJoinLegacy {
	return &NodeJoinLegacy{port, nickname}
}

// Serializable interface: data length when serialized
func (c *NodeJoinLegacy) GetSerializedLength() int {
	return message_fields.SizePort + message_fields.SerializedStringLength(c.Nickname, maximumNicknameLengthBytes)
}

// Serializable interface: convert to bytes.
func (c *NodeJoinLegacy) ToBytes() []byte {
	var serialized []byte
	serialized = append(serialized, message_fields.SerializeInt32(c.Port)...)
	serialized = append(serialized, message_fields.SerializeString(c.Nickname, maximumNicknameLengthBytes)...)
	return serialized
}

// Serializable interface: convert from bytes.
func (c *NodeJoinLegacy) Read(r io.Reader) error {
	var err error
	c.Port, err = message_fields.ReadInt32(r)
	if err != nil {
		return err
	}
	c.Nickname, err = message_fields.ReadString(r)
	return err
}

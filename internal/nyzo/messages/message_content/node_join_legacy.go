/*
TODO: the Java version sends V1 join messages during the startup process, but V2s in some other contexts. Clarify whether this is really needed, or whether we can just live with V2 everywhere.
*/
package message_content

import (
	"errors"
	"github.com/cryptic-monk/go-nyzo/internal/nyzo/messages/message_content/message_fields"
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
func (c *NodeJoinLegacy) FromBytes(bytes []byte) (int, error) {
	if len(bytes) < message_fields.SizePort+message_fields.SizeStringLength {
		return 0, errors.New("invalid node join legacy content")
	}
	position := 0
	c.Port = message_fields.DeserializeInt32(bytes[position : position+message_fields.SizePort])
	position += message_fields.SizePort
	var consumed int
	c.Nickname, consumed = message_fields.DeserializeString(bytes[position:])
	position += consumed
	return consumed, nil
}

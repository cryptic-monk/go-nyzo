package message_content

import (
	"errors"
	"github.com/cryptic-monk/go-nyzo/internal/nyzo/messages/message_content/message_fields"
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
func (c *NodeJoinResponse) FromBytes(bytes []byte) (int, error) {
	if len(bytes) < message_fields.SizeStringLength+message_fields.SizePort*2 {
		return 0, errors.New("invalid node join v2 content")
	}
	position := 0
	var bytesConsumed int
	c.Nickname, bytesConsumed = message_fields.DeserializeString(bytes[position:])
	position += bytesConsumed
	if len(bytes)-bytesConsumed < message_fields.SizePort*2 {
		return position, errors.New("invalid node join v2 content")
	}
	c.PortTcp = message_fields.DeserializeInt32(bytes[position : position+message_fields.SizePort])
	position += message_fields.SizePort
	c.PortUdp = message_fields.DeserializeInt32(bytes[position : position+message_fields.SizePort])
	position += message_fields.SizePort
	return position, nil
}

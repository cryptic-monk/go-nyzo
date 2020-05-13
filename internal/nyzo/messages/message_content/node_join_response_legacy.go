package message_content

import (
	"errors"
	"github.com/cryptic-monk/go-nyzo/internal/nyzo/messages/message_content/message_fields"
)

type NodeJoinResponseLegacy struct {
	Nickname        string
	Port            int32
	NewVerifierVote NewVerifierVote
}

func NewNodeJoinResponseLegacy(nickname string, port int32, newVerifierVote NewVerifierVote) *NodeJoinResponseLegacy {
	return &NodeJoinResponseLegacy{nickname, port, newVerifierVote}
}

// Serializable interface: data length when serialized
func (c *NodeJoinResponseLegacy) GetSerializedLength() int {
	return message_fields.SerializedStringLength(c.Nickname, maximumNicknameLengthBytes) + message_fields.SizePort + message_fields.SizeVoteListLength + c.NewVerifierVote.GetSerializedLength()
}

// Serializable interface: convert to bytes.
func (c *NodeJoinResponseLegacy) ToBytes() []byte {
	var serialized []byte
	serialized = append(serialized, message_fields.SerializeString(c.Nickname, maximumNicknameLengthBytes)...)
	serialized = append(serialized, message_fields.SerializeInt32(c.Port)...)
	serialized = append(serialized, 0) // zero length block vote list, for backwards compatibility
	serialized = append(serialized, c.NewVerifierVote.ToBytes()...)
	return serialized
}

// Serializable interface: convert from bytes.
func (c *NodeJoinResponseLegacy) FromBytes(bytes []byte) (int, error) {
	if len(bytes) < message_fields.SizeStringLength+message_fields.SizePort+message_fields.SizeVoteListLength+c.NewVerifierVote.GetSerializedLength() {
		return 0, errors.New("invalid node join response legacy content")
	}
	position := 0
	var bytesConsumed int
	c.Nickname, bytesConsumed = message_fields.DeserializeString(bytes[position:])
	position += bytesConsumed
	if len(bytes)-position < message_fields.SizePort+message_fields.SizeVoteListLength+c.NewVerifierVote.GetSerializedLength() {
		return position, errors.New("invalid node join response legacy content")
	}
	c.Port = message_fields.DeserializeInt32(bytes[position : position+message_fields.SizePort])
	position += message_fields.SizePort
	// legacy: discard block votes if there are any. we don't even deserialize them
	votes := bytes[position]
	for i := 0; i < int(votes); i++ {
		position += message_fields.SizeBlockHeight + message_fields.SizeHash + message_fields.SizeTimestamp
	}
	c.NewVerifierVote = NewVerifierVote{}
	if len(bytes)-position != c.NewVerifierVote.GetSerializedLength() {
		return position, errors.New("invalid node join response legacy content")
	}
	var consumed int
	consumed, err := c.NewVerifierVote.FromBytes(bytes[position:])
	position += consumed
	return consumed, err
}

package message_content

import (
	"github.com/cryptic-monk/go-nyzo/internal/nyzo/messages/message_content/message_fields"
	"io"
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
func (c *NodeJoinResponseLegacy) Read(r io.Reader) error {
	var err error
	c.Nickname, err = message_fields.ReadString(r)
	if err != nil {
		return err
	}
	c.Port, err = message_fields.ReadInt32(r)
	if err != nil {
		return err
	}
	// legacy: discard block votes if there are any.
	votes, err := message_fields.ReadByte(r)
	if err != nil {
		return err
	}
	for i := 0; i < int(votes); i++ {
		_, err = message_fields.ReadBytes(r, message_fields.SizeBlockHeight+message_fields.SizeHash+message_fields.SizeTimestamp)
		if err != nil {
			return err
		}
	}
	c.NewVerifierVote = NewVerifierVote{}
	err = c.NewVerifierVote.Read(r)
	return err
}

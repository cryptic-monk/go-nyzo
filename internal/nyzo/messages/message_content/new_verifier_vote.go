package message_content

import (
	"errors"
	"github.com/cryptic-monk/go-nyzo/internal/nyzo/messages/message_content/message_fields"
	"github.com/cryptic-monk/go-nyzo/internal/nyzo/utilities"
)

type NewVerifierVote struct {
	Identifier []byte
}

func NewNewVerifierVote(identifier []byte) *NewVerifierVote {
	return &NewVerifierVote{identifier}
}

// Serializable interface: data length when serialized
func (c *NewVerifierVote) GetSerializedLength() int {
	return message_fields.SizeNodeIdentifier
}

// Serializable interface: convert to bytes.
func (c *NewVerifierVote) ToBytes() []byte {
	return c.Identifier
}

// Serializable interface: convert from bytes.
func (c *NewVerifierVote) FromBytes(bytes []byte) (int, error) {
	if len(bytes) != message_fields.SizeNodeIdentifier {
		return 0, errors.New("invalid new verifier vote content")
	}
	c.Identifier = utilities.ByteArrayCopy(bytes, message_fields.SizeNodeIdentifier)
	return message_fields.SizeNodeIdentifier, nil
}

package message_content

import (
	"github.com/cryptic-monk/go-nyzo/internal/nyzo/messages/message_content/message_fields"
	"io"
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
func (c *NewVerifierVote) Read(r io.Reader) error {
	var err error
	c.Identifier, err = message_fields.ReadNodeId(r)
	return err
}

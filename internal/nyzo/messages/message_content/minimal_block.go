package message_content

import (
	"github.com/cryptic-monk/go-nyzo/internal/nyzo/messages/message_content/message_fields"
	"io"
)

type MinimalBlock struct {
	VerificationTimestamp int64
	Signature             []byte
}

func NewMinimalBlock(verificationTimestamp int64, signature []byte) *MinimalBlock {
	return &MinimalBlock{verificationTimestamp, signature}
}

// Serializable interface: data length when serialized
func (c *MinimalBlock) GetSerializedLength() int {
	return message_fields.SizeTimestamp + message_fields.SizeSignature
}

// Serializable interface: convert to bytes.
func (c *MinimalBlock) ToBytes() []byte {
	var serialized []byte
	serialized = append(serialized, message_fields.SerializeInt64(c.VerificationTimestamp)...)
	serialized = append(serialized, c.Signature...)
	return serialized
}

// Serializable interface: convert from bytes.
func (c *MinimalBlock) Read(r io.Reader) error {
	var err error
	c.VerificationTimestamp, err = message_fields.ReadInt64(r)
	if err != nil {
		return err
	}
	c.Signature, err = message_fields.ReadBytes(r, message_fields.SizeSignature)
	if err != nil {
		return err
	}
	return err
}

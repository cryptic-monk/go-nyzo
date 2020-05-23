/*
A block vote.
*/
package blockchain_data

import (
	"github.com/cryptic-monk/go-nyzo/internal/nyzo/messages/message_content/message_fields"
	"io"
)

type BlockVote struct {
	Height           int64
	Hash             []byte
	Timestamp        int64  // to prevent replay attacks of old votes (actually, unnecessary due to message timestamp)
	ReceiptTimestamp int64  // local-only field used to ensure minimum spacing between votes
	SenderIdentifier []byte // sender of the message for BlockWithVotesResponse
	MessageTimestamp int64  // timestamp of the message for BlockWithVotesResponse
	MessageSignature []byte // signature of the message for BlockWithVotesResponse
}

// Serializable interface: data length when serialized
func (b *BlockVote) GetSerializedLength() int {
	return message_fields.SizeBlockHeight + message_fields.SizeHash + message_fields.SizeTimestamp
}

// Serializable interface: convert to bytes.
func (b *BlockVote) ToBytes() []byte {
	var serialized []byte
	serialized = append(serialized, message_fields.SerializeInt64(b.Height)...)
	serialized = append(serialized, b.Hash...)
	serialized = append(serialized, message_fields.SerializeInt64(b.Timestamp)...)
	return serialized
}

// Serializable interface: convert from bytes.
func (b *BlockVote) Read(r io.Reader) error {
	var err error
	b.Height, err = message_fields.ReadInt64(r)
	if err != nil {
		return err
	}
	b.Hash, err = message_fields.ReadHash(r)
	if err != nil {
		return err
	}
	b.Timestamp, err = message_fields.ReadInt64(r)
	if err != nil {
		return err
	}
	return nil
}

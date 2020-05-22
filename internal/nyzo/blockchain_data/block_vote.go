/*
A block vote.
*/
package blockchain_data

import (
	"errors"
	"github.com/cryptic-monk/go-nyzo/internal/nyzo/messages/message_content/message_fields"
	"github.com/cryptic-monk/go-nyzo/internal/nyzo/utilities"
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
func (b *BlockVote) FromBytes(data []byte) (int, error) {
	if len(data) < b.GetSerializedLength() {
		return 0, errors.New("invalid block vote data")
	}
	position := 0
	b.Height = message_fields.DeserializeInt64(data[position : position+message_fields.SizeBlockHeight])
	position += message_fields.SizeBlockHeight
	b.Hash = utilities.ByteArrayCopy(data[position:position+message_fields.SizeHash], position+message_fields.SizeHash)
	position += message_fields.SizeHash
	b.Timestamp = message_fields.DeserializeInt64(data[position : position+message_fields.SizeTimestamp])
	position += message_fields.SizeTimestamp
	return position, nil
}

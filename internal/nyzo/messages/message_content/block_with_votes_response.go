package message_content

import (
	"errors"
	"github.com/cryptic-monk/go-nyzo/internal/nyzo/blockchain_data"
	"github.com/cryptic-monk/go-nyzo/internal/nyzo/messages/message_content/message_fields"
)

type BlockWithVotesResponse struct {
	Block *blockchain_data.Block
	Votes []*blockchain_data.BlockVote
}

// Serializable interface: data length when serialized
func (r *BlockWithVotesResponse) GetSerializedLength() int {
	size := message_fields.SizeBool
	if r.Block != nil {
		size += r.Block.GetSerializedLength()
	}
	size += message_fields.SizeUnnamedInt16
	if r.Votes != nil {
		size += (message_fields.SizeNodeIdentifier + message_fields.SizeTimestamp*2 + message_fields.SizeSignature) * len(r.Votes)
	}
	return size
}

// Serializable interface: convert to bytes.
func (r *BlockWithVotesResponse) ToBytes() []byte {
	var serialized []byte
	serialized = append(serialized, message_fields.SerializeBool(r.Block != nil)...)
	if r.Block != nil {
		serialized = append(serialized, r.Block.ToBytes()...)
	}
	if r.Votes != nil {
		serialized = append(serialized, message_fields.SerializeInt16(int16(len(r.Votes)))...)
		for _, vote := range r.Votes {
			serialized = append(serialized, vote.SenderIdentifier...)
			serialized = append(serialized, message_fields.SerializeInt64(vote.Timestamp)...)
			serialized = append(serialized, message_fields.SerializeInt64(vote.MessageTimestamp)...)
			serialized = append(serialized, vote.MessageSignature...)
		}
	} else {
		serialized = append(serialized, message_fields.SerializeInt16(0)...)
	}
	return serialized
}

// Serializable interface: convert from bytes.
func (r *BlockWithVotesResponse) FromBytes(bytes []byte) (int, error) {
	if len(bytes) < message_fields.SizeBool+message_fields.SizeUnnamedInt16 {
		return 0, errors.New("invalid block votes response content 1")
	}
	position := 0
	hasBlock := message_fields.DeserializeBool(bytes[position : position+message_fields.SizeBool])
	position += message_fields.SizeBool
	if hasBlock {
		block, consumed := blockchain_data.NewBlockFromBytes(bytes[position:])
		if block == nil {
			return 0, errors.New("invalid block votes response content 2")
		}
		r.Block = block
		position += consumed
	}
	if len(bytes)-position < message_fields.SizeUnnamedInt16 {
		return 0, errors.New("invalid block votes response content 3")
	}
	voteCount := message_fields.DeserializeInt16(bytes[position : position+message_fields.SizeUnnamedInt16])
	position += message_fields.SizeUnnamedInt16
	if r.Block != nil {
		r.Votes = make([]*blockchain_data.BlockVote, 0, voteCount)
		for i := 0; i < int(voteCount); i++ {
			if len(bytes)-position < message_fields.SizeNodeIdentifier+message_fields.SizeTimestamp*2+message_fields.SizeSignature {
				return 0, errors.New("invalid block votes response content 4")
			}
			identifier := bytes[position : position+message_fields.SizeNodeIdentifier]
			position += message_fields.SizeNodeIdentifier
			timestamp := message_fields.DeserializeInt64(bytes[position : position+message_fields.SizeTimestamp])
			position += message_fields.SizeTimestamp
			messageTimestamp := message_fields.DeserializeInt64(bytes[position : position+message_fields.SizeTimestamp])
			position += message_fields.SizeTimestamp
			signature := bytes[position : position+message_fields.SizeSignature]
			position += message_fields.SizeSignature
			blockVote := blockchain_data.BlockVote{}
			blockVote.Height = r.Block.Height
			blockVote.Hash = r.Block.Hash
			blockVote.Timestamp = timestamp
			blockVote.SenderIdentifier = identifier
			blockVote.MessageTimestamp = messageTimestamp
			blockVote.MessageSignature = signature
			r.Votes = append(r.Votes, &blockVote)
		}
	}
	return position, nil
}

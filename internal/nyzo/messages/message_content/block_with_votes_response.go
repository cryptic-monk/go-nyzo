package message_content

import (
	"github.com/cryptic-monk/go-nyzo/internal/nyzo/blockchain_data"
	"github.com/cryptic-monk/go-nyzo/internal/nyzo/messages/message_content/message_fields"
	"io"
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
func (r *BlockWithVotesResponse) Read(re io.Reader) error {
	hasBlock, err := message_fields.ReadBool(re)
	if err != nil {
		return err
	}
	if hasBlock {
		r.Block, err = blockchain_data.ReadNewBlock(re)
		if err != nil {
			return err
		}
	}
	voteCount, err := message_fields.ReadInt16(re)
	if err != nil {
		return err
	}
	if r.Block != nil {
		r.Votes = make([]*blockchain_data.BlockVote, 0, voteCount)
		for i := 0; i < int(voteCount); i++ {
			identifier, err := message_fields.ReadNodeId(re)
			if err != nil {
				return err
			}
			timestamp, err := message_fields.ReadInt64(re)
			if err != nil {
				return err
			}
			messageTimestamp, err := message_fields.ReadInt64(re)
			if err != nil {
				return err
			}
			signature, err := message_fields.ReadSignature(re)
			if err != nil {
				return err
			}
			blockVote := blockchain_data.BlockVote{
				Height:           r.Block.Height,
				Hash:             r.Block.Hash,
				Timestamp:        timestamp,
				ReceiptTimestamp: 0,
				SenderIdentifier: identifier,
				MessageTimestamp: messageTimestamp,
				MessageSignature: signature,
			}
			r.Votes = append(r.Votes, &blockVote)
		}
	}
	return nil
}

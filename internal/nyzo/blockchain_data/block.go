/*
A Nyzo block, only low-level serialization/deserialization activity should happen here, plus basical structural
sanity checking. More complex stuff like transaction validation or continuity needs to go further up.
*/
package blockchain_data

import (
	"errors"
	"fmt"
	"github.com/cryptic-monk/go-nyzo/internal/nyzo/configuration"
	"github.com/cryptic-monk/go-nyzo/internal/nyzo/messages/message_content/message_fields"
	"github.com/cryptic-monk/go-nyzo/internal/nyzo/utilities"
	"io"
)

type Block struct {
	BlockchainVersion     int16          // 2 bytes; 16-bit integer of the blockchain version
	Height                int64          // 6 bytes; 48-bit integer block height from the Genesis block, which has a height of 0
	PreviousBlockHash     []byte         // 32 bytes (this is the double-SHA-256 of the previous block signature)
	StartTimestamp        int64          // 8 bytes; 64-bit Unix timestamp of the start of the block, in milliseconds
	VerificationTimestamp int64          // 8 bytes; 64-bit Unix timestamp of when the verifier creates the block, in milliseconds
	Transactions          []*Transaction // 4 bytes for number + variable
	BalanceListHash       []byte         // 32 bytes (this is the double-SHA-256 of the account balance list)
	VerifierIdentifier    []byte         // 32 bytes
	VerifierSignature     []byte         // 64 bytes
	ContinuityState       int
	SignatureState        int
	CycleInformation      *CycleInformation
	Hash                  []byte
	CycleInfoCache        *BlockCycleInfoCache
}

// Read a new block from the given reader.
func ReadNewBlock(r io.Reader) (*Block, error) {
	b := &Block{}
	err := b.Read(r)
	if err != nil {
		return nil, err
	} else {
		return b, nil
	}
}

// Serializable interface: data length when serialized
func (b *Block) GetSerializedLength() int {
	size := message_fields.SizeShlong + message_fields.SizeHash + message_fields.SizeTimestamp + message_fields.SizeTimestamp + message_fields.SizeUnnamedInt32
	for _, transaction := range b.Transactions {
		size += transaction.GetSerializedLength()
	}
	size += message_fields.SizeHash + message_fields.SizeNodeIdentifier + message_fields.SizeSignature
	return size
}

// Serializable interface: convert to bytes.
func (b *Block) ToBytes() []byte {
	return b.Serialize(false)
}

// Serialize (for message passing or signing)
func (b *Block) Serialize(forSigning bool) []byte {
	var serialized []byte
	combined := ToShlong(b.BlockchainVersion, b.Height)
	serialized = append(serialized, message_fields.SerializeInt64(combined)...)
	serialized = append(serialized, b.PreviousBlockHash...)
	serialized = append(serialized, message_fields.SerializeInt64(b.StartTimestamp)...)
	serialized = append(serialized, message_fields.SerializeInt64(b.VerificationTimestamp)...)
	serialized = append(serialized, message_fields.SerializeInt32(int32(len(b.Transactions)))...)
	for _, transaction := range b.Transactions {
		serialized = append(serialized, transaction.ToBytes()...)
	}
	serialized = append(serialized, b.BalanceListHash...)
	if !forSigning {
		serialized = append(serialized, b.VerifierIdentifier...)
		serialized = append(serialized, b.VerifierSignature...)
	} else {
		//REEEE, it took me about an hour to find out that for signing, the block includes zeroed-out bytes for the verifier ID and verifier signature
		dummy := make([]byte, message_fields.SizeNodeIdentifier+message_fields.SizeSignature, message_fields.SizeNodeIdentifier+message_fields.SizeSignature)
		serialized = append(serialized, dummy...)
	}
	return serialized
}

// Serializable interface: read from a reader.
func (b *Block) Read(r io.Reader) error {
	combined, err := message_fields.ReadInt64(r)
	if err != nil {
		return err
	}
	b.BlockchainVersion, b.Height = FromShlong(combined)
	if b.BlockchainVersion < configuration.MinimumBlockchainVersion || b.BlockchainVersion > configuration.MaximumBlockchainVersion {
		return errors.New(fmt.Sprintf("block has unknown blockchain version %d", b.BlockchainVersion))
	}
	b.PreviousBlockHash, err = message_fields.ReadHash(r)
	if err != nil {
		return err
	}
	b.StartTimestamp, err = message_fields.ReadInt64(r)
	if err != nil {
		return err
	}
	b.VerificationTimestamp, err = message_fields.ReadInt64(r)
	if err != nil {
		return err
	}
	transactionCount, err := message_fields.ReadInt32(r)
	if err != nil {
		return err
	}
	b.Transactions = make([]*Transaction, 0, transactionCount)
	for i := 0; i < int(transactionCount); i++ {
		t := &Transaction{}
		err := t.Read(r, false)
		if err != nil {
			return err
		}
		b.Transactions = append(b.Transactions, t)
	}
	b.BalanceListHash, err = message_fields.ReadHash(r)
	if err != nil {
		return err
	}
	b.VerifierIdentifier, err = message_fields.ReadNodeId(r)
	if err != nil {
		return err
	}
	b.VerifierSignature, err = message_fields.ReadSignature(r)
	if err != nil {
		return err
	}
	b.SignatureState = Undetermined
	b.ContinuityState = Undetermined
	doubleSha := utilities.DoubleSha256(b.VerifierSignature)
	b.Hash = doubleSha[:]
	return nil
}

// Sum up all standard transactions in this block.
func (b *Block) StandardTransactionSum() (sum int64) {
	for _, transaction := range b.Transactions {
		if transaction.Type == TransactionTypeStandard {
			sum += transaction.Amount
		}
	}
	return sum
}

/*
A Nyzo block, only low-level serialization/deserialization activity should happen here, plus basical structural
sanity checking. More complex stuff like transaction validation or continuity needs to go further up.
*/
package blockchain_data

import (
	"errors"
	"fmt"
	"github.com/cryptic-monk/go-nyzo/internal/logging"
	"github.com/cryptic-monk/go-nyzo/internal/nyzo/messages/message_content/message_fields"
	"github.com/cryptic-monk/go-nyzo/internal/nyzo/utilities"
	"os"
)

const (
	minimumVerificationInterval = 1500
	minimumBlockchainVersion    = 0
	maximumBlockchainVersion    = 2
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

// Create a new block from the given byte stream
func NewBlockFromBytes(data []byte) (*Block, int) {
	b := &Block{}
	consumed, err := b.FromBytes(data)
	if err != nil {
		logging.ErrorLog.Print(err.Error())
		return nil, 0
	} else {
		return b, consumed
	}
}

// Create a new block from an individual block file (more efficient than reading the whole file first).
func NewBlockFromIndividualFile(f *os.File) (*Block, error) {
	b := &Block{}
	err := b.FromIndividualFile(f)
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

// Serializable interface: convert from bytes.
func (b *Block) FromBytes(data []byte) (int, error) {
	if len(data) < message_fields.SizeShlong+message_fields.SizeHash+message_fields.SizeTimestamp+message_fields.SizeTimestamp+message_fields.SizeUnnamedInt32+message_fields.SizeHash {
		return 0, errors.New("invalid block data 1")
	}
	position := 0
	combined := message_fields.DeserializeInt64(data[position : position+message_fields.SizeShlong])
	position += message_fields.SizeShlong
	b.BlockchainVersion, b.Height = FromShlong(combined)
	if b.BlockchainVersion < minimumBlockchainVersion || b.BlockchainVersion > maximumBlockchainVersion {
		return position, errors.New(fmt.Sprintf("block has unknown blockchain version %d", b.BlockchainVersion))
	}
	b.PreviousBlockHash = utilities.ByteArrayCopy(data[position:position+message_fields.SizeHash], message_fields.SizeHash)
	position += message_fields.SizeHash
	b.StartTimestamp = message_fields.DeserializeInt64(data[position : position+message_fields.SizeTimestamp])
	position += message_fields.SizeTimestamp
	b.VerificationTimestamp = message_fields.DeserializeInt64(data[position : position+message_fields.SizeTimestamp])
	position += message_fields.SizeTimestamp
	transactionCount := int(message_fields.DeserializeInt32(data[position : position+message_fields.SizeUnnamedInt32]))
	position += message_fields.SizeUnnamedInt32
	b.Transactions = make([]*Transaction, 0, transactionCount)
	for i := 0; i < transactionCount; i++ {
		t := &Transaction{}
		consumed, err := t.FromBytes(data[position:], false)
		position += consumed
		if err != nil {
			return position, err
		}
		b.Transactions = append(b.Transactions, t)
	}
	if len(data)-position < message_fields.SizeNodeIdentifier+message_fields.SizeSignature+message_fields.SizeHash {
		return position, errors.New("invalid block data 2")
	}
	b.BalanceListHash = utilities.ByteArrayCopy(data[position:position+message_fields.SizeHash], message_fields.SizeHash)
	position += message_fields.SizeHash
	b.VerifierIdentifier = utilities.ByteArrayCopy(data[position:position+message_fields.SizeNodeIdentifier], message_fields.SizeNodeIdentifier)
	position += message_fields.SizeNodeIdentifier
	b.VerifierSignature = utilities.ByteArrayCopy(data[position:position+message_fields.SizeSignature], message_fields.SizeSignature)
	position += message_fields.SizeSignature
	b.SignatureState = Undetermined
	b.ContinuityState = Undetermined
	doubleSha := utilities.DoubleSha256(b.VerifierSignature)
	b.Hash = doubleSha[:]
	return position, nil
}

// Serializable interface: read from an individual block file.
func (b *Block) FromIndividualFile(f *os.File) error {
	// discard block count, it's an individual block file
	bytes := make([]byte, 2)
	_, err := f.Read(bytes)
	if err != nil {
		return err
	}
	bytes = make([]byte, message_fields.SizeShlong)
	_, err = f.Read(bytes)
	if err != nil {
		return err
	}
	combined := message_fields.DeserializeInt64(bytes)
	b.BlockchainVersion, b.Height = FromShlong(combined)
	if b.BlockchainVersion < minimumBlockchainVersion || b.BlockchainVersion > maximumBlockchainVersion {
		return errors.New(fmt.Sprintf("block has unknown blockchain version %d", b.BlockchainVersion))
	}
	bytes = make([]byte, message_fields.SizeHash)
	_, err = f.Read(bytes)
	if err != nil {
		return err
	}
	b.PreviousBlockHash = bytes
	bytes = make([]byte, message_fields.SizeTimestamp)
	_, err = f.Read(bytes)
	if err != nil {
		return err
	}
	b.StartTimestamp = message_fields.DeserializeInt64(bytes)
	bytes = make([]byte, message_fields.SizeTimestamp)
	_, err = f.Read(bytes)
	if err != nil {
		return err
	}
	b.VerificationTimestamp = message_fields.DeserializeInt64(bytes)
	bytes = make([]byte, message_fields.SizeUnnamedInt32)
	_, err = f.Read(bytes)
	if err != nil {
		return err
	}
	transactionCount := int(message_fields.DeserializeInt32(bytes))
	b.Transactions = make([]*Transaction, 0, transactionCount)
	for i := 0; i < transactionCount; i++ {
		t := &Transaction{}
		err := t.FromFile(f, false)
		if err != nil {
			return err
		}
		b.Transactions = append(b.Transactions, t)
	}
	bytes = make([]byte, message_fields.SizeHash)
	_, err = f.Read(bytes)
	if err != nil {
		return err
	}
	b.BalanceListHash = bytes
	bytes = make([]byte, message_fields.SizeNodeIdentifier)
	_, err = f.Read(bytes)
	if err != nil {
		return err
	}
	b.VerifierIdentifier = bytes
	bytes = make([]byte, message_fields.SizeSignature)
	_, err = f.Read(bytes)
	if err != nil {
		return err
	}
	b.VerifierSignature = bytes
	b.SignatureState = Undetermined
	b.ContinuityState = Undetermined
	doubleSha := utilities.DoubleSha256(b.VerifierSignature)
	b.Hash = doubleSha[:]
	return nil
}

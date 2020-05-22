/*
An individual transaction. Only simple serialization/deserialization and structural sanity checking belongs here.
*/
package blockchain_data

import (
	"bytes"
	"crypto/ed25519"
	"errors"
	"github.com/cryptic-monk/go-nyzo/internal/nyzo/messages/message_content/message_fields"
	"github.com/cryptic-monk/go-nyzo/internal/nyzo/utilities"
	"os"
	"sort"
)

const (
	TransactionTypeCoinGeneration = 0
	TransactionTypeSeed           = 1
	TransactionTypeStandard       = 2
	TransactionTypeCycle          = 3
	TransactionTypeCycleSignature = 4
	TransactionTypeAccountFee     = 97
	TransactionTypeCycleReward    = 98
	TransactionTypeVerifierReward = 99
)

type Transaction struct {
	// in all transactions
	Type        byte   // 1 byte, see above
	Timestamp   int64  // 8 bytes; 64-bit Unix timestamp of the transaction initiation, in milliseconds
	Amount      int64  // 8 bytes; 64-bit amount in micronyzos
	RecipientId []byte // 32 bytes (256-bit public key of the recipient)
	// type 1, 2 and 3
	PreviousHashHeight int64  // 8 bytes; 64-bit index of the block height of the previous-block hash
	PreviousBlockHash  []byte // 32 bytes (SHA-256 of a recent block in the chain), not serialized
	SenderId           []byte // 32 bytes (256-bit public key of the sender)
	SenderData         []byte // up to 32 bytes, length when serialized: 1 byte + actual length
	// type 1 and 2
	Signature []byte // 64 bytes (512-bit signature)
	// type 3
	CycleSignatures []*CycleSignature
	SignatureState  int
	// type 4
	CycleSignatureTransactions []*Transaction
	CycleTransactionVote       bool
	CycleTransactionSignature  []byte
}

func NewTransactionFromBytes(bytes []byte) (*Transaction, int) {
	t := &Transaction{}
	consumed, err := t.FromBytes(bytes, false)
	if err == nil {
		return t, consumed
	} else {
		return nil, consumed
	}
}

// Serializable interface: data length when serialized
func (t *Transaction) GetSerializedLength() int {
	// All transactions begin with a type and timestamp.
	size := message_fields.SizeTransactionType + message_fields.SizeTimestamp

	if t.Type == TransactionTypeCycleSignature {
		size += message_fields.SizeNodeIdentifier + message_fields.SizeBool + message_fields.SizeSignature // verifier (signer) identifier, yes/no, cycle transaction signature
		size += message_fields.SizeSignature                                                               // our signature
	} else {
		size += message_fields.SizeTransactionAmount + message_fields.SizeNodeIdentifier
	}

	if t.Type == TransactionTypeSeed || t.Type == TransactionTypeStandard || t.Type == TransactionTypeCycle {
		size += message_fields.SizeBlockHeight + message_fields.SizeNodeIdentifier
		// sender data length max 32 bytes
		length := len(t.SenderData)
		if length > 32 {
			length = 32
		}
		size += message_fields.SizeUnnamedByte + length

		if t.Type == TransactionTypeCycle {
			// These are stored differently in the v1 and v2 blockchains. The cycleSignatures field is used for
			// the v1 blockchain, and the cycleSignatureTransactions field is used for the v2 blockchain.
			if t.CycleSignatures != nil && len(t.CycleSignatures) > 0 {
				// The v1 blockchain stores identifier and signature for each.
				size += message_fields.SizeUnnamedInt32 + (len(t.CycleSignatures) * (message_fields.SizeNodeIdentifier + message_fields.SizeSignature))
			} else {
				// The v2 blockchain stores timestamp, identifier, vote, and signature for each.
				size += message_fields.SizeUnnamedInt32 + (len(t.CycleSignatureTransactions) * (message_fields.SizeTimestamp + message_fields.SizeNodeIdentifier + message_fields.SizeBool + message_fields.SizeSignature))
			}
		}
	}

	return size
}

// Serializable interface: convert to bytes.
func (t *Transaction) ToBytes() []byte {
	return t.serialize(false)
}

// Serialize (for message passing or signing)
func (t *Transaction) serialize(forSigning bool) []byte {
	var serialized []byte
	serialized = append(serialized, t.Type)
	serialized = append(serialized, message_fields.SerializeInt64(t.Timestamp)...)

	if t.Type == TransactionTypeCoinGeneration || t.Type == TransactionTypeSeed || t.Type == TransactionTypeStandard || t.Type == TransactionTypeCycle {
		serialized = append(serialized, message_fields.SerializeInt64(t.Amount)...)
		serialized = append(serialized, t.RecipientId...)
	} else if t.Type == TransactionTypeCycleSignature {
		serialized = append(serialized, t.SenderId...)
		serialized = append(serialized, message_fields.SerializeBool(t.CycleTransactionVote)...)
		serialized = append(serialized, t.CycleTransactionSignature...)
		if !forSigning {
			serialized = append(serialized, t.Signature...)
		}
	}

	if t.Type == TransactionTypeSeed || t.Type == TransactionTypeStandard || t.Type == TransactionTypeCycle {
		if forSigning {
			serialized = append(serialized, t.PreviousBlockHash...)
		} else {
			serialized = append(serialized, message_fields.SerializeInt64(t.PreviousHashHeight)...)
		}
		serialized = append(serialized, t.SenderId...)
		// sender data length max 32 bytes
		length := len(t.SenderData)
		if length > 32 {
			length = 32
		}
		if forSigning {
			doubleSha := utilities.DoubleSha256(t.SenderData)
			serialized = append(serialized, doubleSha[:]...)
		} else {
			serialized = append(serialized, byte(length))
			serialized = append(serialized, t.SenderData...)
		}
		if !forSigning {
			serialized = append(serialized, t.Signature...)
		}
		// For cycle transactions, order the signatures by verifier identifier. In the v1 blockchain, the
		// cycleSignatures field is used. In the v2 blockchain, the cycleSignatureTransactions field is used.
		if t.Type == TransactionTypeCycle && !forSigning {
			if t.CycleSignatures != nil && len(t.CycleSignatures) > 0 {
				sort.SliceStable(t.CycleSignatures, func(i, j int) bool {
					return utilities.ByteArrayComparator(t.CycleSignatures[i].Id, t.CycleSignatures[j].Id)
				})
				serialized = append(serialized, message_fields.SerializeInt32(int32(len(t.CycleSignatures)))...)
				for _, signature := range t.CycleSignatures {
					serialized = append(serialized, signature.Id...)
					serialized = append(serialized, signature.Signature...)
				}
			} else {
				sort.SliceStable(t.CycleSignatureTransactions, func(i, j int) bool {
					return utilities.ByteArrayComparator(t.CycleSignatureTransactions[i].SenderId, t.CycleSignatureTransactions[j].SenderId)
				})
				serialized = append(serialized, message_fields.SerializeInt32(int32(len(t.CycleSignatureTransactions)))...)
				for _, transaction := range t.CycleSignatureTransactions {
					serialized = append(serialized, message_fields.SerializeInt64(transaction.Timestamp)...)
					serialized = append(serialized, transaction.SenderId...)
					serialized = append(serialized, message_fields.SerializeBool(transaction.CycleTransactionVote)...)
					serialized = append(serialized, transaction.Signature...)
				}
			}
		}

	}

	return serialized
}

// Serializable interface: convert from bytes. Added a parameter here, let's see whether we really need the serializable interface here or can just go with the direct call.
func (t *Transaction) FromBytes(b []byte, balanceListCycleTransaction bool) (int, error) {
	if len(b) < message_fields.SizeTransactionType+message_fields.SizeTimestamp {
		return 0, errors.New("invalid transaction data 1")
	}
	position := 0
	// All transactions start with type and timestamp
	t.Type = b[position]
	position += message_fields.SizeTransactionType
	t.Timestamp = message_fields.DeserializeInt64(b[position : position+message_fields.SizeTimestamp])
	position += message_fields.SizeTimestamp
	t.SignatureState = Undetermined
	// Only cycle signatures don't have an amount and a recipient for now
	if t.Type != TransactionTypeCycleSignature {
		if len(b)-position < message_fields.SizeTransactionAmount+message_fields.SizeNodeIdentifier {
			return 0, errors.New("invalid transaction data 1.1")
		}
		t.Amount = message_fields.DeserializeInt64(b[position : position+message_fields.SizeTransactionAmount])
		position += message_fields.SizeTransactionAmount
		t.RecipientId = utilities.ByteArrayCopy(b[position:position+message_fields.SizeNodeIdentifier], message_fields.SizeNodeIdentifier)
		position += message_fields.SizeNodeIdentifier
	}
	// Coin generation already done here
	if t.Type == TransactionTypeCoinGeneration {
		return position, nil
	}
	if t.Type == TransactionTypeSeed || t.Type == TransactionTypeStandard || t.Type == TransactionTypeCycle {
		if len(b)-position < message_fields.SizeBlockHeight+message_fields.SizeNodeIdentifier+message_fields.SizeUnnamedByte {
			return position, errors.New("invalid transaction data 2")
		}
		t.PreviousHashHeight = message_fields.DeserializeInt64(b[position : position+8])
		position += message_fields.SizeBlockHeight
		t.PreviousBlockHash = make([]byte, 0, 0)
		t.SenderId = utilities.ByteArrayCopy(b[position:position+message_fields.SizeNodeIdentifier], message_fields.SizeNodeIdentifier)
		position += message_fields.SizeNodeIdentifier
		senderDataLength := int(b[position])
		position += message_fields.SizeUnnamedByte
		if senderDataLength > 32 {
			senderDataLength = 32
		}
		if len(b)-position < senderDataLength {
			return position, errors.New("invalid transaction data 3")
		}
		t.SenderData = b[position : position+senderDataLength]
		position += senderDataLength
		if len(b)-position < message_fields.SizeSignature {
			return position, errors.New("invalid transaction data 4")
		}
		t.Signature = utilities.ByteArrayCopy(b[position:position+message_fields.SizeSignature], message_fields.SizeSignature)
		position += message_fields.SizeSignature
		if t.Type == TransactionTypeCycle {
			if len(b)-position < message_fields.SizeUnnamedInt32 {
				return position, errors.New("invalid transaction data 5")
			}
			signatureCount := int(message_fields.DeserializeInt32(b[position : position+message_fields.SizeUnnamedInt32]))
			position += message_fields.SizeUnnamedInt32
			if !balanceListCycleTransaction {
				if len(b)-position < signatureCount*(message_fields.SizeNodeIdentifier+message_fields.SizeSignature) {
					return position, errors.New("invalid transaction data 6")
				}
				t.CycleSignatures = make([]*CycleSignature, 0, signatureCount)
				for i := 0; i < signatureCount; i++ {
					identifier := utilities.ByteArrayCopy(b[position:position+message_fields.SizeNodeIdentifier], message_fields.SizeNodeIdentifier)
					position += message_fields.SizeNodeIdentifier
					signature := utilities.ByteArrayCopy(b[position:position+message_fields.SizeSignature], message_fields.SizeSignature)
					position += message_fields.SizeSignature
					if !bytes.Equal(identifier, t.SenderId) {
						t.CycleSignatures = append(t.CycleSignatures, &CycleSignature{identifier, signature})
					}
				}
			} else {
				t.CycleSignatureTransactions = make([]*Transaction, 0, signatureCount)
				for i := 0; i < signatureCount; i++ {
					if len(b)-position < message_fields.SizeTimestamp+message_fields.SizeNodeIdentifier+message_fields.SizeBool+message_fields.SizeSignature {
						return position, errors.New("invalid cycleSignatureTransaction data 6.1")
					}
					cycleSignatureTransaction := &Transaction{}
					cycleSignatureTransaction.Type = TransactionTypeCycleSignature
					cycleSignatureTransaction.Timestamp = message_fields.DeserializeInt64(b[position : position+message_fields.SizeTimestamp])
					position += message_fields.SizeTimestamp
					cycleSignatureTransaction.SenderId = utilities.ByteArrayCopy(b[position:position+message_fields.SizeNodeIdentifier], message_fields.SizeNodeIdentifier)
					position += message_fields.SizeNodeIdentifier
					cycleSignatureTransaction.CycleTransactionVote = message_fields.DeserializeBool(b[position : position+message_fields.SizeBool])
					position += message_fields.SizeBool
					cycleSignatureTransaction.Signature = utilities.ByteArrayCopy(b[position:position+message_fields.SizeSignature], message_fields.SizeSignature)
					position += message_fields.SizeSignature
					t.CycleSignatureTransactions = append(t.CycleSignatureTransactions, cycleSignatureTransaction)
				}
			}
		}
	} else if t.Type == TransactionTypeCycleSignature {
		if len(b)-position < message_fields.SizeNodeIdentifier+message_fields.SizeBool+(message_fields.SizeSignature*2) {
			return position, errors.New("invalid transaction data 7")
		}
		t.SenderId = utilities.ByteArrayCopy(b[position:position+message_fields.SizeNodeIdentifier], message_fields.SizeNodeIdentifier)
		position += message_fields.SizeNodeIdentifier
		t.CycleTransactionVote = message_fields.DeserializeBool(b[position : position+message_fields.SizeBool])
		position += message_fields.SizeBool
		t.CycleTransactionSignature = utilities.ByteArrayCopy(b[position:position+message_fields.SizeSignature], message_fields.SizeSignature)
		position += message_fields.SizeSignature
		t.Signature = utilities.ByteArrayCopy(b[position:position+message_fields.SizeSignature], message_fields.SizeSignature)
		position += message_fields.SizeSignature
	} else {
		return position, errors.New("invalid transaction data 8, unknown transaction type")
	}
	return position, nil
}

// Serializable interface: read from a file (more memory efficient than reading the whole file first).
func (t *Transaction) FromFile(f *os.File, balanceListCycleTransaction bool) error {
	b := make([]byte, message_fields.SizeTransactionType)
	_, err := f.Read(b)
	if err != nil {
		return err
	}
	t.Type = b[0]
	b = make([]byte, message_fields.SizeTimestamp)
	_, err = f.Read(b)
	if err != nil {
		return err
	}
	t.Timestamp = message_fields.DeserializeInt64(b)
	t.SignatureState = Undetermined
	// Only cycle signatures don't have an amount and a recipient for now
	if t.Type != TransactionTypeCycleSignature {
		b = make([]byte, message_fields.SizeTransactionAmount)
		_, err = f.Read(b)
		if err != nil {
			return err
		}
		t.Amount = message_fields.DeserializeInt64(b)
		b = make([]byte, message_fields.SizeNodeIdentifier)
		_, err = f.Read(b)
		if err != nil {
			return err
		}
		t.RecipientId = b
	}
	// Coin generation already done here
	if t.Type == TransactionTypeCoinGeneration {
		return nil
	}
	if t.Type == TransactionTypeSeed || t.Type == TransactionTypeStandard || t.Type == TransactionTypeCycle {
		b = make([]byte, message_fields.SizeBlockHeight)
		_, err = f.Read(b)
		if err != nil {
			return err
		}
		t.PreviousHashHeight = message_fields.DeserializeInt64(b)
		t.PreviousBlockHash = make([]byte, 0, 0)
		b = make([]byte, message_fields.SizeNodeIdentifier)
		_, err = f.Read(b)
		if err != nil {
			return err
		}
		t.SenderId = b
		b = make([]byte, message_fields.SizeUnnamedByte)
		_, err = f.Read(b)
		if err != nil {
			return err
		}
		senderDataLength := int(b[0])
		if senderDataLength > 32 {
			senderDataLength = 32
		}
		b = make([]byte, senderDataLength)
		_, err = f.Read(b)
		if err != nil {
			return err
		}
		t.SenderData = b
		b = make([]byte, message_fields.SizeSignature)
		_, err = f.Read(b)
		if err != nil {
			return err
		}
		t.Signature = b
		if t.Type == TransactionTypeCycle {
			b = make([]byte, message_fields.SizeUnnamedInt32)
			_, err = f.Read(b)
			if err != nil {
				return err
			}
			signatureCount := int(message_fields.DeserializeInt32(b))
			if !balanceListCycleTransaction {
				t.CycleSignatures = make([]*CycleSignature, 0, signatureCount)
				for i := 0; i < signatureCount; i++ {
					b = make([]byte, message_fields.SizeNodeIdentifier)
					_, err = f.Read(b)
					if err != nil {
						return err
					}
					identifier := b
					b = make([]byte, message_fields.SizeSignature)
					_, err = f.Read(b)
					if err != nil {
						return err
					}
					signature := b
					if !bytes.Equal(identifier, t.SenderId) {
						t.CycleSignatures = append(t.CycleSignatures, &CycleSignature{identifier, signature})
					}
				}
			} else {
				t.CycleSignatureTransactions = make([]*Transaction, 0, signatureCount)
				for i := 0; i < signatureCount; i++ {
					cycleSignatureTransaction := &Transaction{}
					cycleSignatureTransaction.Type = TransactionTypeCycleSignature
					b = make([]byte, message_fields.SizeTimestamp)
					_, err = f.Read(b)
					if err != nil {
						return err
					}
					cycleSignatureTransaction.Timestamp = message_fields.DeserializeInt64(b)
					b = make([]byte, message_fields.SizeNodeIdentifier)
					_, err = f.Read(b)
					if err != nil {
						return err
					}
					cycleSignatureTransaction.SenderId = b
					b = make([]byte, message_fields.SizeBool)
					_, err = f.Read(b)
					if err != nil {
						return err
					}
					cycleSignatureTransaction.CycleTransactionVote = message_fields.DeserializeBool(b)
					b = make([]byte, message_fields.SizeSignature)
					_, err = f.Read(b)
					if err != nil {
						return err
					}
					cycleSignatureTransaction.Signature = b
					t.CycleSignatureTransactions = append(t.CycleSignatureTransactions, cycleSignatureTransaction)
				}
			}
		}
	} else if t.Type == TransactionTypeCycleSignature {
		b = make([]byte, message_fields.SizeNodeIdentifier)
		_, err = f.Read(b)
		if err != nil {
			return err
		}
		t.SenderId = b
		b = make([]byte, message_fields.SizeBool)
		_, err = f.Read(b)
		if err != nil {
			return err
		}
		t.CycleTransactionVote = message_fields.DeserializeBool(b)
		b = make([]byte, message_fields.SizeSignature)
		_, err = f.Read(b)
		if err != nil {
			return err
		}
		t.CycleTransactionSignature = b
		b = make([]byte, message_fields.SizeSignature)
		_, err = f.Read(b)
		if err != nil {
			return err
		}
		t.Signature = b
	} else {
		return errors.New("invalid transaction data 8, unknown transaction type")
	}
	return nil
}

// Calculate the fee charged for this transaction.
func (t *Transaction) GetFee() int64 {
	if t.Type == TransactionTypeCycle || t.Type == TransactionTypeCoinGeneration || t.Type == TransactionTypeCycleSignature {
		return 0
	} else {
		return (t.Amount + 399) / 400
	}
}

// Verify this transaction's signature (if necessary) and return whether it's valid.
func (t *Transaction) SignatureIsValid() bool {
	if t.SignatureState == Undetermined && (t.Type == TransactionTypeSeed || t.Type == TransactionTypeStandard || t.Type == TransactionTypeCycle || t.Type == TransactionTypeCycleSignature) {
		if ed25519.Verify(t.SenderId, t.serialize(true), t.Signature) {
			t.SignatureState = Valid
		} else {
			t.SignatureState = Invalid
		}
	}
	// Coin generation transactions are only allowed in the genesis block, so we can consider these valid if they exist.
	// Respectively, we'll check elsewhere that there are none in any block above 0.
	if t.Type == TransactionTypeCoinGeneration {
		t.SignatureState = Valid
	}
	return t.SignatureState == Valid
}

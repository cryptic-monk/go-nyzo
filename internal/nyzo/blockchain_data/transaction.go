/*
An individual transaction. Only simple serialization/deserialization and structural sanity checking belongs here.
*/
package blockchain_data

import (
	"bytes"
	"crypto/ed25519"
	"errors"
	"github.com/cryptic-monk/go-nyzo/internal/nyzo/configuration"
	"github.com/cryptic-monk/go-nyzo/internal/nyzo/messages/message_content/message_fields"
	"github.com/cryptic-monk/go-nyzo/internal/nyzo/utilities"
	"io"
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
	Type        byte   `json:"type"`         // 1 byte, see above
	Timestamp   int64  `json:"timestamp"`    // 8 bytes; 64-bit Unix timestamp of the transaction initiation, in milliseconds
	Amount      int64  `json:"amount"`       // 8 bytes; 64-bit amount in micronyzos
	RecipientId []byte `json:"recipient_id"` // 32 bytes (256-bit public key of the recipient)
	// type 1, 2 and 3
	PreviousHashHeight int64  `json:"previous_hash_height"` // 8 bytes; 64-bit index of the block height of the previous-block hash
	PreviousBlockHash  []byte `json:"previous_block_hash"`  // 32 bytes (SHA-256 of a recent block in the chain), not serialized
	SenderId           []byte `json:"sender_id"`            // 32 bytes (256-bit public key of the sender)
	SenderData         []byte `json:"sender_data"`          // up to 32 bytes, length when serialized: 1 byte + actual length
	// type 1 and 2
	Signature []byte `json:"signature"` // 64 bytes (512-bit signature)
	// type 3
	CycleSignatures []*CycleSignature `json:"cycle_signatures"`
	SignatureState  int               `json:"signature_state"`
	// type 4
	CycleSignatureTransactions []*Transaction `json:"cycle_signature_transactions"`
	CycleTransactionVote       bool           `json:"cycle_transaction_vote"`
	CycleTransactionSignature  []byte         `json:"cycle_transaction_signature"`
}

func ReadNewTransaction(r io.Reader) (*Transaction, error) {
	t := &Transaction{}
	err := t.Read(r, false)
	if err != nil {
		return nil, err
	} else {
		return t, nil
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
		size += message_fields.SizeSignature

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
	return t.Serialize(false)
}

// Serialize (for message passing or signing)
func (t *Transaction) Serialize(forSigning bool) []byte {
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

// Serializable interface: read from reader.
func (t *Transaction) Read(r io.Reader, balanceListCycleTransaction bool) error {
	var err error
	t.Type, err = message_fields.ReadByte(r)
	if err != nil {
		return err
	}
	t.Timestamp, err = message_fields.ReadInt64(r)
	if err != nil {
		return err
	}
	t.SignatureState = Undetermined
	// Only cycle signatures don't have an amount and a recipient for now
	if t.Type != TransactionTypeCycleSignature {
		t.Amount, err = message_fields.ReadInt64(r)
		if err != nil {
			return err
		}
		t.RecipientId, err = message_fields.ReadNodeId(r)
		if err != nil {
			return err
		}
	}
	// Coin generation already done here
	if t.Type == TransactionTypeCoinGeneration {
		return nil
	}
	if t.Type == TransactionTypeSeed || t.Type == TransactionTypeStandard || t.Type == TransactionTypeCycle {
		t.PreviousHashHeight, err = message_fields.ReadInt64(r)
		if err != nil {
			return err
		}
		t.PreviousBlockHash = make([]byte, 0, 0)
		t.SenderId, err = message_fields.ReadNodeId(r)
		if err != nil {
			return err
		}
		ub, err := message_fields.ReadByte(r)
		if err != nil {
			return err
		}
		senderDataLength := int(ub)
		if senderDataLength > 32 {
			senderDataLength = 32
		}
		t.SenderData, err = message_fields.ReadBytes(r, int64(senderDataLength))
		if err != nil {
			return err
		}
		t.Signature, err = message_fields.ReadSignature(r)
		if err != nil {
			return err
		}
		if t.Type == TransactionTypeCycle {
			ui, err := message_fields.ReadInt32(r)
			if err != nil {
				return err
			}
			signatureCount := int(ui)
			if !balanceListCycleTransaction {
				t.CycleSignatures = make([]*CycleSignature, 0, signatureCount)
				for i := 0; i < signatureCount; i++ {
					identifier, err := message_fields.ReadNodeId(r)
					if err != nil {
						return err
					}
					signature, err := message_fields.ReadSignature(r)
					if err != nil {
						return err
					}
					if !bytes.Equal(identifier, t.SenderId) {
						t.CycleSignatures = append(t.CycleSignatures, &CycleSignature{identifier, signature})
					}
				}
			} else {
				t.CycleSignatureTransactions = make([]*Transaction, 0, signatureCount)
				for i := 0; i < signatureCount; i++ {
					cycleSignatureTransaction := &Transaction{}
					cycleSignatureTransaction.Type = TransactionTypeCycleSignature
					cycleSignatureTransaction.Timestamp, err = message_fields.ReadInt64(r)
					if err != nil {
						return err
					}
					cycleSignatureTransaction.SenderId, err = message_fields.ReadNodeId(r)
					if err != nil {
						return err
					}
					cycleSignatureTransaction.CycleTransactionVote, err = message_fields.ReadBool(r)
					if err != nil {
						return err
					}
					cycleSignatureTransaction.Signature, err = message_fields.ReadSignature(r)
					if err != nil {
						return err
					}
					t.CycleSignatureTransactions = append(t.CycleSignatureTransactions, cycleSignatureTransaction)
				}
			}
		}
	} else if t.Type == TransactionTypeCycleSignature {
		t.SenderId, err = message_fields.ReadNodeId(r)
		if err != nil {
			return err
		}
		t.CycleTransactionVote, err = message_fields.ReadBool(r)
		if err != nil {
			return err
		}
		t.CycleTransactionSignature, err = message_fields.ReadSignature(r)
		if err != nil {
			return err
		}
		t.Signature, err = message_fields.ReadSignature(r)
		if err != nil {
			return err
		}
	} else {
		return errors.New("invalid transaction data, unknown transaction type")
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
		if ed25519.Verify(t.SenderId, t.Serialize(true), t.Signature) {
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

// Verify whether the given identifier has signed the transaction with the given signature (used for cycle signatures).
func (t *Transaction) ValidSignatureBy(id []byte, signature []byte) bool {
	return ed25519.Verify(id, t.Serialize(true), signature)
}

// Dev account locking mechanism.
func (t *Transaction) IsSubjectToLock() bool {
	if t.Type != TransactionTypeCoinGeneration &&
		t.Type != TransactionTypeSeed &&
		configuration.IsLockedAccount(t.SenderId) &&
		!bytes.Equal(t.RecipientId, configuration.CycleAccount) {
		return true
	}
	return false
}

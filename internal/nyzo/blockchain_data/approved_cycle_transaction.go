/*
Provides a reduced record of approved cycle transactions to store with the balance list. This is used
to enforce the ∩100,000 per 10,000-block limit of NTTP-3/3. The initiator and receiver, while not necessary for
this, are stored because they are small and helpful for anyone reviewing the balance list.
*/
package blockchain_data

import (
	"errors"
	"github.com/cryptic-monk/go-nyzo/internal/nyzo/messages/message_content/message_fields"
	"github.com/cryptic-monk/go-nyzo/internal/nyzo/utilities"
)

type ApprovedCycleTransaction struct {
	InitiatorIdentifier []byte
	ReceiverIdentifier  []byte
	ApprovalHeight      int64
	Amount              int64
}

// Convenience to create an approved cycle transaction from a byte buffer.
func NewApprovedCycleTransactionFromBytes(bytes []byte) (*ApprovedCycleTransaction, int) {
	t := &ApprovedCycleTransaction{}
	consumed, err := t.FromBytes(bytes)
	if err == nil {
		return t, consumed
	} else {
		return nil, consumed
	}
}

// Serializable interface: data length when serialized.
func (t *ApprovedCycleTransaction) GetSerializedLength() int {
	return message_fields.SizeNodeIdentifier*2 + message_fields.SizeBlockHeight + message_fields.SizeTransactionAmount
}

// Serializable interface: convert to bytes.
func (t *ApprovedCycleTransaction) ToBytes() []byte {
	var serialized []byte
	serialized = append(serialized, t.InitiatorIdentifier...)
	serialized = append(serialized, t.ReceiverIdentifier...)
	serialized = append(serialized, message_fields.SerializeInt64(t.ApprovalHeight)...)
	serialized = append(serialized, message_fields.SerializeInt64(t.Amount)...)
	return serialized
}

// Serializable interface: convert from bytes.
func (t *ApprovedCycleTransaction) FromBytes(b []byte) (int, error) {
	if len(b) < t.GetSerializedLength() {
		return 0, errors.New("invalid approved cycle transaction data")
	}
	position := 0
	t.InitiatorIdentifier = utilities.ByteArrayCopy(b[position:position+message_fields.SizeNodeIdentifier], message_fields.SizeNodeIdentifier)
	position += message_fields.SizeNodeIdentifier
	t.ReceiverIdentifier = utilities.ByteArrayCopy(b[position:position+message_fields.SizeNodeIdentifier], message_fields.SizeNodeIdentifier)
	position += message_fields.SizeNodeIdentifier
	t.ApprovalHeight = message_fields.DeserializeInt64(b[position : position+message_fields.SizeBlockHeight])
	position += message_fields.SizeBlockHeight
	t.Amount = message_fields.DeserializeInt64(b[position : position+message_fields.SizeTransactionAmount])
	position += message_fields.SizeTransactionAmount
	return position, nil
}

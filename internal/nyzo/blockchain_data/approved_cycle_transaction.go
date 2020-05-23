/*
Provides a reduced record of approved cycle transactions to store with the balance list. This is used
to enforce the ∩100,000 per 10,000-block limit of NTTP-3/3. The initiator and receiver, while not necessary for
this, are stored because they are small and helpful for anyone reviewing the balance list.
*/
package blockchain_data

import (
	"github.com/cryptic-monk/go-nyzo/internal/nyzo/messages/message_content/message_fields"
	"io"
)

type ApprovedCycleTransaction struct {
	InitiatorIdentifier []byte
	ReceiverIdentifier  []byte
	ApprovalHeight      int64
	Amount              int64
}

// Convenience to create an approved cycle transaction from a byte buffer.
func ReadNewApprovedCycleTransaction(r io.Reader) (*ApprovedCycleTransaction, error) {
	t := &ApprovedCycleTransaction{}
	err := t.Read(r)
	if err != nil {
		return nil, err
	} else {
		return t, nil
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
func (t *ApprovedCycleTransaction) Read(r io.Reader) error {
	var err error
	t.InitiatorIdentifier, err = message_fields.ReadNodeId(r)
	if err != nil {
		return err
	}
	t.ReceiverIdentifier, err = message_fields.ReadNodeId(r)
	if err != nil {
		return err
	}
	t.ApprovalHeight, err = message_fields.ReadInt64(r)
	if err != nil {
		return err
	}
	t.Amount, err = message_fields.ReadInt64(r)
	return err
}

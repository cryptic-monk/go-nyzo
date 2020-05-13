/*
A balance list, including all the auxiliary data typical for the Java implementation. This is not strictly
part of the blockchain, but for local storage, balance lists are serialized and deserialized just like
blockchain data.
*/
package blockchain_data

import (
	"bytes"
	"errors"
	"github.com/cryptic-monk/go-nyzo/internal/logging"
	"github.com/cryptic-monk/go-nyzo/internal/nyzo/messages/message_content/message_fields"
	"github.com/cryptic-monk/go-nyzo/internal/nyzo/utilities"
	"sort"
)

type BalanceList struct {
	BlockchainVersion                 int16
	BlockHeight                       int64
	RolloverFees                      byte
	PreviousVerifiers                 [][]byte
	Items                             []BalanceListItem
	UnlockThreshold                   int64
	UnlockTransferSum                 int64
	PendingCycleTransactions          []*Transaction
	RecentlyApprovedCycleTransactions []*ApprovedCycleTransaction
}

func NewBalanceListFromBytes(data []byte) (*BalanceList, int) {
	bl := &BalanceList{}
	consumed, err := bl.FromBytes(data)
	if err != nil {
		logging.ErrorLog.Print(err.Error())
		return nil, 0
	} else {
		return bl, consumed
	}
}

// Normalize a balance list by removing accounts with 0 micronyzo and duplicates
func (bl *BalanceList) Normalize() {
	// Java sorts "to make removal of duplicates easier", we TRY to replicate this here in case it matters for serialization.
	// This is likely imprecise as Java and Go would have to be more profoundly compared to find out how exactly they deal
	// with stable/unstable sort orders of equal items.
	sort.SliceStable(bl.Items, func(i, j int) bool {
		return utilities.ByteArrayComparator(bl.Items[i].Identifier, bl.Items[j].Identifier)
	})

	for i := len(bl.Items) - 1; i >= 0; i-- {
		if bl.Items[i].Balance <= 0 {
			copy(bl.Items[i:], bl.Items[i+1:])
			bl.Items = bl.Items[:len(bl.Items)-1]
		}
	}

	for i := len(bl.Items) - 1; i >= 1; i-- {
		if bytes.Equal(bl.Items[i].Identifier, bl.Items[i-1].Identifier) {
			copy(bl.Items[i:], bl.Items[i+1:])
			bl.Items = bl.Items[:len(bl.Items)-1]
		}
	}
}

// Get the hash for this balance list
func (bl *BalanceList) GetHash() []byte {
	doubleSha256 := utilities.DoubleSha256(bl.ToBytes())
	return doubleSha256[:]
}

// Serializable interface: data length when serialized
func (bl *BalanceList) GetSerializedLength() int {
	numberOfPreviousVerifiers := bl.BlockHeight
	if numberOfPreviousVerifiers > 9 {
		numberOfPreviousVerifiers = 9
	}
	size := message_fields.SizeShlong + message_fields.SizeRolloverTransactionFee
	size += int(numberOfPreviousVerifiers) * message_fields.SizeNodeIdentifier
	size += message_fields.SizeBalanceListLength
	size += len(bl.Items) * (message_fields.SizeNodeIdentifier + message_fields.SizeTransactionAmount + message_fields.SizeBlocksUntilFee)
	if bl.BlockchainVersion > 0 {
		size += message_fields.SizeTransactionAmount * 2
	}
	if bl.BlockchainVersion > 1 {
		size += message_fields.SizeUnnamedInt32
		for _, transaction := range bl.PendingCycleTransactions {
			size += transaction.GetSerializedLength()
		}
		size += message_fields.SizeUnnamedInt32
		for _, transaction := range bl.RecentlyApprovedCycleTransactions {
			size += transaction.GetSerializedLength()
		}
	}
	return size
}

// Serializable interface: convert to bytes.
func (bl *BalanceList) ToBytes() []byte {
	var serialized []byte
	combined := ToShlong(bl.BlockchainVersion, bl.BlockHeight)
	serialized = append(serialized, message_fields.SerializeInt64(combined)...)
	serialized = append(serialized, bl.RolloverFees)
	for _, id := range bl.PreviousVerifiers {
		serialized = append(serialized, id...)
	}
	serialized = append(serialized, message_fields.SerializeInt32(int32(len(bl.Items)))...)
	for _, item := range bl.Items {
		serialized = append(serialized, item.Identifier...)
		serialized = append(serialized, message_fields.SerializeInt64(item.Balance)...)
		serialized = append(serialized, message_fields.SerializeInt16(item.BlocksUntilFee)...)
	}
	if bl.BlockchainVersion > 0 {
		serialized = append(serialized, message_fields.SerializeInt64(bl.UnlockThreshold)...)
		serialized = append(serialized, message_fields.SerializeInt64(bl.UnlockTransferSum)...)
	}
	if bl.BlockchainVersion > 1 {
		sort.SliceStable(bl.PendingCycleTransactions, func(i, j int) bool {
			return utilities.ByteArrayComparator(bl.PendingCycleTransactions[i].SenderId, bl.PendingCycleTransactions[j].SenderId)
		})
		serialized = append(serialized, message_fields.SerializeInt32(int32(len(bl.PendingCycleTransactions)))...)
		for _, transaction := range bl.PendingCycleTransactions {
			serialized = append(serialized, transaction.ToBytes()...)
		}
		serialized = append(serialized, message_fields.SerializeInt32(int32(len(bl.RecentlyApprovedCycleTransactions)))...)
		for _, transaction := range bl.RecentlyApprovedCycleTransactions {
			serialized = append(serialized, transaction.ToBytes()...)
		}
	}
	return serialized
}

// Serialization interface. Read a balance list from bytes.
func (bl *BalanceList) FromBytes(data []byte) (int, error) {
	if len(data) < message_fields.SizeShlong+message_fields.SizeUnnamedByte {
		return 0, errors.New("invalid balance list data 1")
	}
	position := 0
	combined := message_fields.DeserializeInt64(data[position : position+message_fields.SizeShlong])
	position += message_fields.SizeShlong
	version, height := FromShlong(combined)
	bl.BlockchainVersion = version
	bl.BlockHeight = height
	bl.RolloverFees = data[position]
	position += message_fields.SizeRolloverTransactionFee
	numberOfPreviousVerifiers := bl.BlockHeight
	if numberOfPreviousVerifiers > 9 {
		numberOfPreviousVerifiers = 9
	}
	if len(data)-position < (int(numberOfPreviousVerifiers)*message_fields.SizeNodeIdentifier)+message_fields.SizeBalanceListLength {
		return position, errors.New("invalid balance list data 2")
	}
	bl.PreviousVerifiers = make([][]byte, 0, numberOfPreviousVerifiers)
	for i := 0; i < int(numberOfPreviousVerifiers); i++ {
		bl.PreviousVerifiers = append(bl.PreviousVerifiers, data[position:position+message_fields.SizeNodeIdentifier])
		position += message_fields.SizeNodeIdentifier
	}
	numberOfEntries := message_fields.DeserializeInt32(data[position : position+message_fields.SizeBalanceListLength])
	position += message_fields.SizeBalanceListLength
	if len(data)-position < int(numberOfEntries)*(message_fields.SizeNodeIdentifier+message_fields.SizeTransactionAmount+message_fields.SizeBlocksUntilFee) {
		return position, errors.New("invalid balance list data 3")
	}
	bl.Items = make([]BalanceListItem, 0, numberOfEntries)
	for i := 0; i < int(numberOfEntries); i++ {
		id := data[position : position+message_fields.SizeNodeIdentifier]
		position += message_fields.SizeNodeIdentifier
		balance := message_fields.DeserializeInt64(data[position : position+message_fields.SizeTransactionAmount])
		position += message_fields.SizeTransactionAmount
		blockUntilFee := message_fields.DeserializeInt16(data[position : position+message_fields.SizeUnnamedInt16])
		position += message_fields.SizeBlocksUntilFee
		bl.Items = append(bl.Items, BalanceListItem{id, balance, blockUntilFee})
	}
	bl.UnlockThreshold = 0
	bl.UnlockTransferSum = 0
	if bl.BlockchainVersion > 0 {
		if len(data)-position < message_fields.SizeTransactionAmount*2 {
			return position, errors.New("invalid balance list data 4")
		}
		bl.UnlockThreshold = message_fields.DeserializeInt64(data[position : position+message_fields.SizeTransactionAmount])
		position += message_fields.SizeTransactionAmount
		bl.UnlockTransferSum = message_fields.DeserializeInt64(data[position : position+message_fields.SizeTransactionAmount])
		position += message_fields.SizeTransactionAmount
	}
	if bl.BlockchainVersion > 1 {
		// blockchain v2 and above: read cycle transaction and approved cycle transaction info
		if len(data)-position < message_fields.SizeUnnamedInt32 {
			return position, errors.New("invalid balance list data 5")
		}
		numberOfEntries := message_fields.DeserializeInt32(data[position : position+message_fields.SizeUnnamedInt32])
		position += message_fields.SizeUnnamedInt32
		bl.PendingCycleTransactions = make([]*Transaction, 0, numberOfEntries)
		for i := 0; i < int(numberOfEntries); i++ {
			t := &Transaction{}
			consumed, err := t.FromBytes(data[position:], true)
			position += consumed
			if err != nil {
				return position, err
			}
			bl.PendingCycleTransactions = append(bl.PendingCycleTransactions, t)
		}
		if len(data)-position < message_fields.SizeUnnamedInt32 {
			return position, errors.New("invalid balance list data 5")
		}
		numberOfEntries = message_fields.DeserializeInt32(data[position : position+message_fields.SizeUnnamedInt32])
		position += message_fields.SizeUnnamedInt32
		bl.RecentlyApprovedCycleTransactions = make([]*ApprovedCycleTransaction, 0, numberOfEntries)
		for i := 0; i < int(numberOfEntries); i++ {
			t := &ApprovedCycleTransaction{}
			consumed, err := t.FromBytes(data[position:])
			position += consumed
			if err != nil {
				return position, err
			}
			bl.RecentlyApprovedCycleTransactions = append(bl.RecentlyApprovedCycleTransactions, t)
		}

	}
	return position, nil
}

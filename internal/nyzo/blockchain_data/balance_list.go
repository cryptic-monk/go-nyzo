/*
A balance list, including all the auxiliary data typical for the Java implementation. This is not strictly
part of the blockchain, but for local storage, balance lists are serialized and deserialized just like
blockchain data.
*/
package blockchain_data

import (
	"bytes"
	"github.com/cryptic-monk/go-nyzo/internal/nyzo/configuration"
	"github.com/cryptic-monk/go-nyzo/internal/nyzo/messages/message_content/message_fields"
	"github.com/cryptic-monk/go-nyzo/internal/nyzo/utilities"
	"io"
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

func ReadNewBalanceList(r io.Reader) (*BalanceList, error) {
	bl := &BalanceList{}
	err := bl.Read(r)
	if err != nil {
		return nil, err
	} else {
		return bl, nil
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
func (bl *BalanceList) Read(r io.Reader) error {
	combined, err := message_fields.ReadInt64(r)
	if err != nil {
		return err
	}
	version, height := FromShlong(combined)
	bl.BlockchainVersion = version
	bl.BlockHeight = height
	bl.RolloverFees, err = message_fields.ReadByte(r)
	if err != nil {
		return err
	}
	numberOfPreviousVerifiers := bl.BlockHeight
	if numberOfPreviousVerifiers > 9 {
		numberOfPreviousVerifiers = 9
	}
	bl.PreviousVerifiers = make([][]byte, 0, numberOfPreviousVerifiers)
	for i := 0; i < int(numberOfPreviousVerifiers); i++ {
		id, err := message_fields.ReadNodeId(r)
		if err != nil {
			return err
		}
		bl.PreviousVerifiers = append(bl.PreviousVerifiers, id)
	}
	numberOfEntries, err := message_fields.ReadInt32(r)
	if err != nil {
		return err
	}
	bl.Items = make([]BalanceListItem, 0, numberOfEntries)
	for i := 0; i < int(numberOfEntries); i++ {
		id, err := message_fields.ReadNodeId(r)
		if err != nil {
			return err
		}
		balance, err := message_fields.ReadInt64(r)
		if err != nil {
			return err
		}
		blockUntilFee, err := message_fields.ReadInt16(r)
		if err != nil {
			return err
		}
		bl.Items = append(bl.Items, BalanceListItem{id, balance, blockUntilFee})
	}
	bl.UnlockThreshold = 0
	bl.UnlockTransferSum = 0
	if bl.BlockchainVersion > 0 {
		bl.UnlockThreshold, err = message_fields.ReadInt64(r)
		if err != nil {
			return err
		}
		bl.UnlockTransferSum, err = message_fields.ReadInt64(r)
		if err != nil {
			return err
		}
	}
	if bl.BlockchainVersion > 1 {
		// blockchain v2 and above: read cycle transaction and approved cycle transaction info
		numberOfEntries, err := message_fields.ReadInt32(r)
		if err != nil {
			return err
		}
		bl.PendingCycleTransactions = make([]*Transaction, 0, numberOfEntries)
		for i := 0; i < int(numberOfEntries); i++ {
			t := &Transaction{}
			err = t.Read(r, true)
			if err != nil {
				return err
			}
			bl.PendingCycleTransactions = append(bl.PendingCycleTransactions, t)
		}
		numberOfEntries, err = message_fields.ReadInt32(r)
		if err != nil {
			return err
		}
		bl.RecentlyApprovedCycleTransactions = make([]*ApprovedCycleTransaction, 0, numberOfEntries)
		for i := 0; i < int(numberOfEntries); i++ {
			t := &ApprovedCycleTransaction{}
			err = t.Read(r)
			if err != nil {
				return err
			}
			bl.RecentlyApprovedCycleTransactions = append(bl.RecentlyApprovedCycleTransactions, t)
		}

	}
	return nil
}

// Does the given id exist in the balance list?
func (bl *BalanceList) HasAccount(id []byte) bool {
	for _, item := range bl.Items {
		if bytes.Equal(item.Identifier, id) {
			return true
		}
	}
	return false
}

// Get balance for the given ID.
func (bl *BalanceList) GetBalance(id []byte) int64 {
	for _, item := range bl.Items {
		if bytes.Equal(item.Identifier, id) {
			return item.Balance
		}
	}
	return 0
}

// Adjust the balance list entry with the given account id by the given amount (adding an item to the list if necessary). Returns the amount after adjustment.
func (bl *BalanceList) AdjustBalance(id []byte, amount int64) int64 {
	// do return adjusted balance even if the amount is 0, but...
	for i, item := range bl.Items {
		if bytes.Equal(item.Identifier, id) {
			item.Balance += amount
			bl.Items[i] = item
			return item.Balance
		}
	}
	// ...don't create entries for 0 balances
	if amount == 0 {
		return 0
	}
	item := BalanceListItem{
		Identifier:     id,
		Balance:        amount,
		BlocksUntilFee: configuration.BlocksBetweenFee,
	}
	if bytes.Equal(id, configuration.TransferAccount) {
		item.BlocksUntilFee = 0
	}
	bl.Items = append(bl.Items, item)
	return amount
}

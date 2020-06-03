/*
Handles block to block balance list updates, which includes processing of V2 cycle transactions, and emission
of transactions if so desired.
*/
package balance_authority

import (
	"bytes"
	"encoding/json"
	"fmt"
	"github.com/cryptic-monk/go-nyzo/internal/logging"
	"github.com/cryptic-monk/go-nyzo/internal/nyzo/blockchain_data"
	"github.com/cryptic-monk/go-nyzo/internal/nyzo/configuration"
	"github.com/cryptic-monk/go-nyzo/internal/nyzo/interfaces"
	"github.com/cryptic-monk/go-nyzo/internal/nyzo/messages"
	"github.com/cryptic-monk/go-nyzo/internal/nyzo/messages/message_content/message_fields"
	"github.com/cryptic-monk/go-nyzo/internal/nyzo/router"
	"github.com/cryptic-monk/go-nyzo/internal/nyzo/utilities"
	"github.com/cryptic-monk/go-nyzo/pkg/identity"
	"sort"
)

// Update the <balanceList> from the block preceding <thisBlock> to contain the transactions from <thisBlock>.
// <previousVerifier> is the verifier for the previous block (used for reward distribution). Requires the
// <ctxt> to check on V2 type cycle signatures, but is stateless in itself. Set <emitTransactions> to true for
// transaction data emission (archive node).
func UpdateBalanceListForNextBlock(ctxt *interfaces.Context, previousVerifier []byte, balanceList *blockchain_data.BalanceList, thisBlock *blockchain_data.Block, emitTransactions bool) *blockchain_data.BalanceList {
	if balanceList == nil {
		// genesis block: create a new balance list
		balanceList = &blockchain_data.BalanceList{}
		balanceList.PreviousVerifiers = make([][]byte, 0, 0)
		balanceList.Items = make([]blockchain_data.BalanceListItem, 0, 0)
		balanceList.PendingCycleTransactions = make([]*blockchain_data.Transaction, 0, 0)
		balanceList.RecentlyApprovedCycleTransactions = make([]*blockchain_data.ApprovedCycleTransaction, 0, 0)
	} else {
		// all other blocks: copy the balance list before updating it
		b := balanceList.ToBytes()
		newBalanceList := &blockchain_data.BalanceList{}
		_ = newBalanceList.Read(bytes.NewReader(b))
		balanceList = newBalanceList
		// make sure we have these ready in case the blockchain version goes above 1 below
		if balanceList.PendingCycleTransactions == nil {
			balanceList.PendingCycleTransactions = make([]*blockchain_data.Transaction, 0, 0)
		}
		if balanceList.RecentlyApprovedCycleTransactions == nil {
			balanceList.RecentlyApprovedCycleTransactions = make([]*blockchain_data.ApprovedCycleTransaction, 0, 0)
		}
	}
	balanceList.BlockHeight = thisBlock.Height
	balanceList.BlockchainVersion = thisBlock.BlockchainVersion

	// beyond genesis block: maintain list of previous verifiers for reward distribution
	if previousVerifier != nil {
		balanceList.PreviousVerifiers = append(balanceList.PreviousVerifiers, previousVerifier)
		if len(balanceList.PreviousVerifiers) > 9 {
			balanceList.PreviousVerifiers = balanceList.PreviousVerifiers[1:]
		}
	}

	//TODO: past the frozenEdge, we have to verify/approve the transactions for the given block here

	var feesThisBlock, organicTransactionFees, transactionSumFromLockedAccounts int64
	var senderBalance, recipientBalance, amount, amountAfterFee *int64
	var canonical int32 // canonical order in block, starting with 0
	var vote *byte      // cycle signature transactions
	for _, transaction := range thisBlock.Transactions {
		// In blockchain version 1, cycle transactions are processed here. In later versions, cycle
		// transactions are incorporated in the blockchain before being approved for transfer, so they are
		// handled separately.
		if transaction.Type != blockchain_data.TransactionTypeCycle || thisBlock.BlockchainVersion == 1 {
			// the cycle account has an imaginary id with no known private key
			var senderId []byte
			if transaction.Type == blockchain_data.TransactionTypeCycle {
				senderId, _ = identity.NyzoHexToBytes([]byte(configuration.CycleAccountNyzoHex), message_fields.SizeNodeIdentifier)
			} else {
				senderId = transaction.SenderId
			}
			// adjust sender account
			if transaction.Type != blockchain_data.TransactionTypeCoinGeneration && transaction.Type != blockchain_data.TransactionTypeCycleSignature {
				senderBalance = new(int64)
				*senderBalance = adjustBalance(balanceList, senderId, -transaction.Amount)
			} else {
				senderBalance = nil
			}
			// determine amounts and adjust recipient account
			if transaction.Type != blockchain_data.TransactionTypeCycleSignature {
				amount = new(int64)
				amountAfterFee = new(int64)
				recipientBalance = new(int64)
				*amount = transaction.Amount
				*amountAfterFee = transaction.Amount - transaction.GetFee()
				*recipientBalance = adjustBalance(balanceList, transaction.RecipientId, *amountAfterFee)
			} else {
				amount = nil
				amountAfterFee = nil
				recipientBalance = nil
			}
			// determine vote for cycle signature transactions
			if transaction.Type == blockchain_data.TransactionTypeCycleSignature {
				vote = new(byte)
				if transaction.CycleTransactionVote {
					*vote = 1
				}
			} else {
				vote = nil
			}
			if emitTransactions {
				canonical = emitTransaction(thisBlock.Height, canonical, transaction.Timestamp, transaction.Type, senderId, transaction.RecipientId, transaction.SenderData, amount, amountAfterFee, senderBalance, recipientBalance, transaction.Signature, vote, transaction.CycleTransactionSignature)
			}
			// keep track of fees
			feesThisBlock += transaction.GetFee()
			if transaction.Type == blockchain_data.TransactionTypeStandard {
				organicTransactionFees += transaction.GetFee()
			}
			// keep track of locked account transactions
			if transaction.Type != blockchain_data.TransactionTypeCoinGeneration &&
				transaction.Type != blockchain_data.TransactionTypeSeed &&
				configuration.IsLockedAccount(transaction.SenderId) &&
				!bytes.Equal(transaction.RecipientId, configuration.CycleAccount) {
				transactionSumFromLockedAccounts += transaction.Amount
			}
		}
	}

	// Process cycle and cycle-signature transactions in version 2 or later.
	if thisBlock.BlockchainVersion >= 2 {
		canonical = processV2CycleTransactions(ctxt, canonical, balanceList, thisBlock, emitTransactions)
	}

	// send 1% of organic fees to cycle account
	canonical = -1 // negative canonicals for pseudo transactions
	if thisBlock.BlockchainVersion > 0 && organicTransactionFees >= 100 {
		cycleTransferAmount := new(int64)
		recipientBalance := new(int64)
		*cycleTransferAmount = organicTransactionFees / 100
		*recipientBalance = adjustBalance(balanceList, configuration.CycleAccount, *cycleTransferAmount)
		feesThisBlock -= *cycleTransferAmount
		if emitTransactions {
			canonical = emitTransaction(thisBlock.Height, canonical, thisBlock.StartTimestamp, byte(blockchain_data.TransactionTypeCycleReward), nil, configuration.CycleAccount, nil, cycleTransferAmount, cycleTransferAmount, nil, recipientBalance, nil, nil, nil)
		}
	}

	// charge fees on accounts
	var periodicAccountFees int64
	for i, item := range balanceList.Items {
		// In version 0 of the blockchain, charge μ1 every 500 blocks. In version 1 of the blockchain,
		// charge μ100 every 500 blocks for all accounts less than ∩1. Always reset the fee counter.
		if item.BlocksUntilFee <= 0 && !bytes.Equal(item.Identifier, configuration.TransferAccount) {
			item.BlocksUntilFee = configuration.BlocksBetweenFee
			if thisBlock.BlockchainVersion == 0 && item.Balance > 0 {
				item.Balance -= 1
				periodicAccountFees++
				if emitTransactions {
					feeAmount := new(int64)
					balanceAfterFee := new(int64)
					*feeAmount = -1
					*balanceAfterFee = item.Balance
					canonical = emitTransaction(thisBlock.Height, canonical, thisBlock.StartTimestamp, byte(blockchain_data.TransactionTypeAccountFee), nil, configuration.CycleAccount, nil, feeAmount, feeAmount, nil, balanceAfterFee, nil, nil, nil)
				}
			} else if (thisBlock.BlockchainVersion == 1 || thisBlock.BlockchainVersion > 2) && item.Balance < configuration.MicronyzoMultiplierRatio {
				fee := item.Balance
				if fee > 100 {
					fee = 100
				}
				item.Balance -= fee
				periodicAccountFees += fee
				if emitTransactions {
					feeAmount := new(int64)
					balanceAfterFee := new(int64)
					*feeAmount = -fee
					*balanceAfterFee = item.Balance
					canonical = emitTransaction(thisBlock.Height, canonical, thisBlock.StartTimestamp, byte(blockchain_data.TransactionTypeAccountFee), nil, configuration.CycleAccount, nil, feeAmount, feeAmount, nil, balanceAfterFee, nil, nil, nil)
				}
			}
		}
		balanceList.Items[i] = item
	}

	// Split the transaction fees among the current and previous verifiers.
	totalFees := feesThisBlock + int64(balanceList.RolloverFees) + periodicAccountFees
	feesPerVerifier := new(int64)
	*feesPerVerifier = totalFees / int64(len(balanceList.PreviousVerifiers)+1)
	if *feesPerVerifier > 0 {
		recipientBalance := new(int64)
		*recipientBalance = adjustBalance(balanceList, thisBlock.VerifierIdentifier, *feesPerVerifier)
		if emitTransactions {
			canonical = emitTransaction(thisBlock.Height, canonical, thisBlock.StartTimestamp, byte(blockchain_data.TransactionTypeVerifierReward), nil, thisBlock.VerifierIdentifier, nil, feesPerVerifier, feesPerVerifier, nil, recipientBalance, nil, nil, nil)
		}
		for _, previousVerifier := range balanceList.PreviousVerifiers {
			recipientBalance := new(int64)
			*recipientBalance = adjustBalance(balanceList, previousVerifier, *feesPerVerifier)
			if emitTransactions {
				canonical = emitTransaction(thisBlock.Height, canonical, thisBlock.StartTimestamp, byte(blockchain_data.TransactionTypeVerifierReward), nil, previousVerifier, nil, feesPerVerifier, feesPerVerifier, nil, recipientBalance, nil, nil, nil)
			}
		}
	}

	// Make a new list of balance list items: remove 0 items, decrease blocks until fee.
	var micronyzosInSystem int64
	newBalanceListItems := balanceList.Items[:0]
	for _, item := range balanceList.Items {
		if item.Balance > 0 {
			item.BlocksUntilFee--
			if item.BlocksUntilFee < 0 {
				item.BlocksUntilFee = 0
			}
			micronyzosInSystem += item.Balance
			newBalanceListItems = append(newBalanceListItems, item)
		}
	}
	balanceList.Items = newBalanceListItems

	// make sure that we don't "loose" any micronyzos during the above fee distribution
	rolloverFees := byte(totalFees % int64(len(balanceList.PreviousVerifiers)+1))
	micronyzosInSystem += int64(rolloverFees)
	balanceList.RolloverFees = rolloverFees

	// unlocking process of locked dev accounts based on organic transaction fees
	if thisBlock.BlockchainVersion == 0 {
		balanceList.UnlockThreshold = 0
		balanceList.UnlockTransferSum = 0
	} else {
		balanceList.UnlockThreshold += organicTransactionFees
		balanceList.UnlockTransferSum += transactionSumFromLockedAccounts
	}

	// final check, after all this, the overall amount of micronyzos in the system must still be the same as always
	if micronyzosInSystem == configuration.MicronyzosInSystem {
		balanceList.Normalize()
		return balanceList
	} else {
		output, _ := json.Marshal(balanceList)
		fmt.Println(string(output))
		logging.ErrorLog.Fatalf("Could not update balance list to height %d, micronyzos in system value incorrect, have: %d, should: %d. Rollover fees: %d. Dumped full balance list to stdout.", balanceList.BlockHeight, micronyzosInSystem, configuration.MicronyzosInSystem, balanceList.RolloverFees)
		return nil
	}
}

// Adjust the balance list entry with the given account id by the given amount (adding an item to the list if necessary). Returns the amount after adjustment.
func adjustBalance(balanceList *blockchain_data.BalanceList, id []byte, amount int64) int64 {
	// do return adjusted balance even if the amount is 0, but...
	for i, item := range balanceList.Items {
		if bytes.Equal(item.Identifier, id) {
			item.Balance += amount
			balanceList.Items[i] = item
			return item.Balance
		}
	}
	// ...don't create entries for 0 balances
	if amount == 0 {
		return 0
	}
	item := blockchain_data.BalanceListItem{
		Identifier:     id,
		Balance:        amount,
		BlocksUntilFee: configuration.BlocksBetweenFee,
	}
	if bytes.Equal(id, configuration.TransferAccount) {
		item.BlocksUntilFee = 0
	}
	balanceList.Items = append(balanceList.Items, item)
	return amount
}

// Get balance for the given ID, only used here for now.
func getBalance(balanceList *blockchain_data.BalanceList, id []byte) int64 {
	for _, item := range balanceList.Items {
		if bytes.Equal(item.Identifier, id) {
			return item.Balance
		}
	}
	return 0
}

// Process V2 cycle transactions and signatures as part of the above UpdateBalanceListForNextBlock.
func processV2CycleTransactions(ctxt *interfaces.Context, canonical int32, balanceList *blockchain_data.BalanceList, thisBlock *blockchain_data.Block, emitTransactions bool) int32 {
	// Add new cycle transactions to the pending transactions.
	for _, transaction := range thisBlock.Transactions {
		if transaction.Type == blockchain_data.TransactionTypeCycle {
			found := false
			for i, pendingTransaction := range balanceList.PendingCycleTransactions {
				if bytes.Equal(pendingTransaction.SenderId, transaction.SenderId) {
					balanceList.PendingCycleTransactions[i] = transaction
					found = true
					break
				}
			}
			if !found {
				balanceList.PendingCycleTransactions = append(balanceList.PendingCycleTransactions, transaction)
			}
			// Emit the transaction.
			if emitTransactions {
				amount := new(int64)
				*amount = transaction.Amount
				canonical = emitTransaction(thisBlock.Height, canonical, transaction.Timestamp, transaction.Type, configuration.CycleAccount, transaction.RecipientId, transaction.SenderData, amount, amount, nil, nil, transaction.Signature, nil, nil)
			}
		}
	}

	// Remove any out-of-cycle transactions from the map.
	// Slice filtering as per: https://github.com/golang/go/wiki/SliceTricks#filtering-without-allocating.
	newPendingTransactions := balanceList.PendingCycleTransactions[:0]
	for _, pendingTransaction := range balanceList.PendingCycleTransactions {
		if ctxt.CycleAuthority.VerifierInCurrentCycle(pendingTransaction.SenderId) {
			newPendingTransactions = append(newPendingTransactions, pendingTransaction)
		}
	}
	balanceList.PendingCycleTransactions = newPendingTransactions

	// Add all cycle-signature transactions from this block to their parent transactions.
	for _, transaction := range thisBlock.Transactions {
		if transaction.Type == blockchain_data.TransactionTypeCycleSignature {
			for _, pendingTransaction := range balanceList.PendingCycleTransactions {
				if bytes.Equal(pendingTransaction.Signature, transaction.CycleTransactionSignature) {
					found := false
					for i, signatureTransaction := range pendingTransaction.CycleSignatureTransactions {
						if bytes.Equal(signatureTransaction.SenderId, transaction.SenderId) {
							pendingTransaction.CycleSignatureTransactions[i] = transaction
							found = true
							break
						}
					}
					if !found {
						pendingTransaction.CycleSignatureTransactions = append(pendingTransaction.CycleSignatureTransactions, transaction)
					}
				}
			}
		}
	}

	// Remove all out-of-cycle signatures from pending cycle transactions.
	for _, pendingTransaction := range balanceList.PendingCycleTransactions {
		newSignatureTransactions := pendingTransaction.CycleSignatureTransactions[:0]
		for _, signatureTransaction := range pendingTransaction.CycleSignatureTransactions {
			if ctxt.CycleAuthority.VerifierInCurrentCycle(signatureTransaction.SenderId) {
				newSignatureTransactions = append(newSignatureTransactions, signatureTransaction)
			}
		}
		pendingTransaction.CycleSignatureTransactions = newSignatureTransactions
	}

	// Remove recently approved transactions that have surpassed the retention threshold.
	newApprovedTransactions := balanceList.RecentlyApprovedCycleTransactions[:0]
	for _, approvedTransaction := range balanceList.RecentlyApprovedCycleTransactions {
		if approvedTransaction.ApprovalHeight >= thisBlock.Height-configuration.ApprovedCycleTransactionRetentionInterval {
			newApprovedTransactions = append(newApprovedTransactions, approvedTransaction)
		}
	}
	balanceList.RecentlyApprovedCycleTransactions = newApprovedTransactions

	// Sum the recently approved transactions and calculate the maximum allowable cycle transaction
	// amount for this block.
	recentCycleTransactionSum := int64(0)
	for _, approvedTransaction := range balanceList.RecentlyApprovedCycleTransactions {
		recentCycleTransactionSum += approvedTransaction.Amount
	}
	cycleAccountBalance := getBalance(balanceList, configuration.CycleAccount)
	maximumCycleTransactionAmount := configuration.MaximumCycleTransactionAmount - recentCycleTransactionSum
	if cycleAccountBalance < maximumCycleTransactionAmount {
		maximumCycleTransactionAmount = cycleAccountBalance
	}
	if thisBlock.Height%1000 == 0 {
		logging.TraceLog.Printf("Maximum cycle transaction amount=%d, balance=%d.", maximumCycleTransactionAmount, cycleAccountBalance)
	}

	// Get up to one approved cycle transaction in this block. Cycle transaction approval is such a big
	// event that allowing more than one per block is not necessary. For consistency, these are
	// examined in identifier order, and the first transaction with enough votes and a suitable amount
	// is selected.
	var approvedCycleTransaction *blockchain_data.Transaction = nil
	sort.SliceStable(balanceList.PendingCycleTransactions, func(i, j int) bool {
		return utilities.ByteArrayComparator(balanceList.PendingCycleTransactions[i].SenderId, balanceList.PendingCycleTransactions[j].SenderId)
	})
	voteThreshold := ctxt.CycleAuthority.GetCurrentCycleLength()/2 + 1
	for _, pendingTransaction := range balanceList.PendingCycleTransactions {
		if len(pendingTransaction.CycleSignatureTransactions) >= voteThreshold && pendingTransaction.Amount <= maximumCycleTransactionAmount {
			yesVoteCount := 0
			for _, signatureTransaction := range pendingTransaction.CycleSignatureTransactions {
				if signatureTransaction.CycleTransactionVote == true {
					yesVoteCount++
				}
			}
			if yesVoteCount >= voteThreshold {
				approvedCycleTransaction = pendingTransaction
				break
			}
		}
	}

	if approvedCycleTransaction != nil {
		// Remove the transaction from the pending list.
		logging.InfoLog.Printf("Approved cycle transaction at height %d.", thisBlock.Height)
		newPendingTransactions := balanceList.PendingCycleTransactions[:0]
		for _, pendingTransaction := range balanceList.PendingCycleTransactions {
			if !bytes.Equal(pendingTransaction.SenderId, approvedCycleTransaction.SenderId) {
				newPendingTransactions = append(newPendingTransactions, pendingTransaction)
			}
		}
		balanceList.PendingCycleTransactions = newPendingTransactions
		// Add an entry to the recently approved list.
		approvedTransaction := &blockchain_data.ApprovedCycleTransaction{
			InitiatorIdentifier: approvedCycleTransaction.SenderId,
			ReceiverIdentifier:  approvedCycleTransaction.RecipientId,
			ApprovalHeight:      thisBlock.Height,
			Amount:              approvedCycleTransaction.Amount,
		}
		balanceList.RecentlyApprovedCycleTransactions = append(balanceList.RecentlyApprovedCycleTransactions, approvedTransaction)
		// Adjust the balance of the cycle account and the receiver account.
		senderBalance := new(int64)
		recipientBalance := new(int64)
		amount := new(int64)
		*senderBalance = adjustBalance(balanceList, configuration.CycleAccount, -approvedCycleTransaction.Amount)
		*recipientBalance = adjustBalance(balanceList, approvedCycleTransaction.RecipientId, approvedCycleTransaction.Amount)
		*amount = approvedCycleTransaction.Amount
		// Emit the transaction.
		if emitTransactions {
			canonical = emitTransaction(thisBlock.Height, canonical, approvedCycleTransaction.Timestamp, approvedCycleTransaction.Type, configuration.CycleAccount, approvedCycleTransaction.RecipientId, approvedCycleTransaction.SenderData, amount, amount, senderBalance, recipientBalance, approvedCycleTransaction.Signature, nil, nil)
		}
	}

	return canonical
}

// Emit a transaction message (to be caught by the DB store normally).
func emitTransaction(height int64, canonical int32, timestamp int64, transactionType byte, sender, recipient, data []byte, amount, amountAfterFee, senderBalance, recipientBalance *int64, signature []byte, vote *byte, originalSignature []byte) int32 {
	message := messages.NewInternalMessage(messages.TypeInternalTransaction, height, canonical, timestamp, transactionType, sender, recipient, data, amount, amountAfterFee, senderBalance, recipientBalance, signature, vote, originalSignature)
	router.Router.RouteInternal(message)
	if transactionType < blockchain_data.TransactionTypeAccountFee {
		canonical++
	} else {
		canonical--
	}
	return canonical
}

// Does the given id exist in the balance list?
func hasAccount(balanceList *blockchain_data.BalanceList, id []byte) bool {
	for _, item := range balanceList.Items {
		if bytes.Equal(item.Identifier, id) {
			return true
		}
	}
	return false
}

// To prevent issues related to an exceptionally large balance list, some limitations are needed to avoid the
// creation of many small accounts. There are two ways to create many accounts with little funds: directly, by
// transferring a small amount to a new account, and indirectly, by transferring a larger amount away from an
// account to create a new account, leaving very little in the source account. Both of these cases are addressed
// here.
//
// balanceList is the balance list form the previous block.
func transactionSpamsBalanceList(balanceList *blockchain_data.BalanceList, transaction *blockchain_data.Transaction, allTransactionsInBlock []*blockchain_data.Transaction) bool {
	var isSpam bool

	// Only standard transactions are of concern.
	if transaction.Type == blockchain_data.TransactionTypeStandard {

		// This is the direct case. A ∩10 transaction will produce a new account of slightly less than ∩10, but this
		// is not an issue. We are simply trying to make it difficult to spam the balance list. The exact threshold
		// is less important than having a threshold significantly more than μ1, and a minimum transaction of ∩10
		// for a new account is less confusing than a minimum of ∩10.025063. A transaction of only μ1 will not spam
		// the balance list, as the full transaction amount is consumed by the transaction fee, and a new entry is
		// not created in the balance list.
		if !hasAccount(balanceList, transaction.RecipientId) && transaction.Amount > 1 && transaction.Amount < configuration.MinimumPreferredBalance {
			isSpam = true
		} else {
			// This is the indirect case. The existing account needs to have at least ∩10 in it or be empty after
			// the block. All transactions must be considered, or multiple transactions could be sent from a single
			// account to bypass the rule.
			senderBalance := getBalance(balanceList, transaction.SenderId)
			var senderSum int64
			for _, blockTransaction := range allTransactionsInBlock {
				if bytes.Equal(blockTransaction.SenderId, transaction.SenderId) {
					senderSum += blockTransaction.Amount
				}
			}
			if senderBalance-senderSum < configuration.MinimumPreferredBalance && senderBalance-senderSum != 0 {
				isSpam = true
			}
		}
	}

	return isSpam
}

// Counts transaction spam in the given block, balanceList is the balance list of the previous block.
func NumberOfTransactionsSpammingBalanceList(balanceList *blockchain_data.BalanceList, transactions []*blockchain_data.Transaction) int {
	var count int
	for _, transaction := range transactions {
		if transactionSpamsBalanceList(balanceList, transaction, transactions) {
			count++
		}
	}
	return count
}

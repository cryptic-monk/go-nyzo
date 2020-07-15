/*
Manage transactions.
*/
package transaction_manager

import (
	"bytes"
	"github.com/cryptic-monk/go-nyzo/internal/logging"
	"github.com/cryptic-monk/go-nyzo/internal/nyzo/blockchain_data"
	"github.com/cryptic-monk/go-nyzo/internal/nyzo/configuration"
	"github.com/cryptic-monk/go-nyzo/internal/nyzo/interfaces"
	"github.com/cryptic-monk/go-nyzo/internal/nyzo/messages"
	"github.com/cryptic-monk/go-nyzo/internal/nyzo/messages/message_content/message_fields"
	"github.com/cryptic-monk/go-nyzo/internal/nyzo/router"
	"github.com/cryptic-monk/go-nyzo/internal/nyzo/utilities"
	"os"
	"sort"
	"sync"
)

type state struct {
	ctxt                         *interfaces.Context
	internalMessageChannel       chan *messages.InternalMessage // channel for internal and local messages
	frozenEdgeHeight             int64
	chainInitialized             bool
	seedTransactionCacheLock     sync.Mutex
	seedTransactionCache         map[int64]*blockchain_data.Transaction
	highestCachedSeedTransaction int64
}

// Returns only valid transactions from the given list, including signature, duplicate and malleability check.
// Hence the startTimestamp: only transactions between startTimestamp and startTimestamp + BlockDuration are allowed into a block.
// This function merges Block.validTransactions and BalanceManager.transactionsWithoutDuplicates from Java.
func (s *state) ValidTransactionsOnly(transactions []*blockchain_data.Transaction, startTimestamp int64) []*blockchain_data.Transaction {
	validTransactions := make([]*blockchain_data.Transaction, 0, len(transactions))
	observedSignatures := make([][]byte, 0, len(transactions))
	observedRawBytes := make([][]byte, 0, len(transactions))
	endTimestamp := startTimestamp + configuration.BlockDuration
	for _, transaction := range transactions {
		if (transaction.Type == blockchain_data.TransactionTypeCoinGeneration || transaction.SignatureIsValid()) && transaction.Timestamp >= startTimestamp && transaction.Timestamp < endTimestamp {
			var id []byte
			if transaction.Type == blockchain_data.TransactionTypeCoinGeneration {
				id = make([]byte, message_fields.SizeHash, message_fields.SizeHash)
			} else {
				id = transaction.Signature
			}
			raw := transaction.Serialize(true)
			if !utilities.ByteArrayContains(observedSignatures, id) && !utilities.ByteArrayContains(observedRawBytes, raw) {
				observedSignatures = append(observedSignatures, id)
				observedRawBytes = append(observedRawBytes, raw)
				validTransactions = append(validTransactions, transaction)
			}
		}
	}
	return validTransactions
}

func (s *state) ApprovedTransactionsForBlock(transactions []*blockchain_data.Transaction, previousBlock *blockchain_data.Block, forBlockAssembly bool) []*blockchain_data.Transaction {
	// Sort in block order.
	sort.SliceStable(transactions, func(i, j int) bool {
		return blockOrderComparator(transactions[i], transactions[j])
	})
	// First cleanup.
	blockHeight := previousBlock.Height + 1
	transactions = s.ValidTransactionsOnly(transactions, s.ctxt.BlockAuthority.GetGenesisBlockTimestamp()+blockHeight*configuration.BlockDuration)
	// Remove invalid types and transactions below 1 micronyzo
	// TODO: Java checks previousHashIsValid too, but the function always just returns true. Research why this is still there.
	var validTypes map[byte]bool
	if blockHeight == 0 {
		validTypes = map[byte]bool{blockchain_data.TransactionTypeCoinGeneration: true, blockchain_data.TransactionTypeSeed: true, blockchain_data.TransactionTypeStandard: true}
	} else if previousBlock.BlockchainVersion == 0 {
		validTypes = map[byte]bool{blockchain_data.TransactionTypeSeed: true, blockchain_data.TransactionTypeStandard: true}
	} else if previousBlock.BlockchainVersion == 1 {
		validTypes = map[byte]bool{blockchain_data.TransactionTypeSeed: true, blockchain_data.TransactionTypeStandard: true, blockchain_data.TransactionTypeCycle: true}
	} else {
		validTypes = map[byte]bool{blockchain_data.TransactionTypeSeed: true, blockchain_data.TransactionTypeStandard: true, blockchain_data.TransactionTypeCycle: true, blockchain_data.TransactionTypeCycleSignature: true}
	}
	newTransactions := transactions[:0]
	for _, transaction := range transactions {
		// type
		_, ok := validTypes[transaction.Type]
		// size
		if transaction.Type != blockchain_data.TransactionTypeCycleSignature && transaction.Amount < 1 {
			ok = false
		}
		if ok {
			newTransactions = append(newTransactions, transaction)
		} else {
			logging.InfoLog.Println("removed transaction because type or amount is invalid")
		}
	}
	// Protection for seed funds, cycle transactions and locked accounts.
	transactions = newTransactions
	transactions = protectSeedFundingAccount(s.ctxt, transactions, blockHeight)
	transactions = enforceCycleTransactionRules(s.ctxt, transactions, previousBlock.BlockchainVersion)
	// Get the balance list and copy it so that the below operations don't impact other parts of the system.
	balanceList := s.ctxt.BlockHandler.GetBalanceListForBlock(previousBlock).Copy()
	transactions = enforceLockingRules(transactions, balanceList.BlockchainVersion, balanceList.UnlockThreshold, balanceList.UnlockTransferSum)
	// Don't allow sending more than the account owns.
	newTransactions = transactions[:0]
	for _, transaction := range transactions {
		var senderId []byte
		if transaction.Type == blockchain_data.TransactionTypeCycle {
			senderId = configuration.CycleAccount
		} else {
			senderId = transaction.SenderId
		}
		senderBalance := balanceList.GetBalance(senderId)
		if transaction.Amount <= senderBalance || (transaction.Type == blockchain_data.TransactionTypeSeed && transaction.GetFee() < senderBalance) {
			newTransactions = append(newTransactions, transaction)
			balanceList.AdjustBalance(senderId, -transaction.Amount)
			balanceList.AdjustBalance(transaction.RecipientId, transaction.Amount-transaction.GetFee())
		} else {
			logging.InfoLog.Println("removed transaction because sender doesn't have enough funds")
		}
	}
	transactions = newTransactions
	// If this method is being used for new-block assembly and the list of approved transactions is larger than
	// allowed for the block, remove the smallest transactions until the list is an acceptable size.
	excessTransactions := len(transactions) - s.ctxt.CycleAuthority.GetMaximumTransactionsForBlockAssembly()
	if excessTransactions > 0 && forBlockAssembly {
		// Sort the transactions on amount descending, promoting cycle-signature transactions to the head of the list.
		sort.SliceStable(transactions, func(i, j int) bool {
			return !amountComparator(transactions[i], transactions[j])
		})
		// Remove the tail of the list.
		transactions = transactions[:len(transactions)-excessTransactions]
		// Sort the list back to block order.
		sort.SliceStable(transactions, func(i, j int) bool {
			return blockOrderComparator(transactions[i], transactions[j])
		})
	}
	return transactions
}

// At block 1, 20% of the coins in the system were transferred to the seed-funding account. All of the seed
// transactions were pre-signed, and the private key for the account was never saved. However, there is no
// way to prove that the private key was not saved, so this logic provides assurance that the funds in that
// account will only be used for the published seed transactions.
func protectSeedFundingAccount(ctxt *interfaces.Context, transactions []*blockchain_data.Transaction, blockHeight int64) []*blockchain_data.Transaction {
	// These are the same parameters used to generate the transactions. In addition to transfers, funds
	// could be stolen from this account with large seed transactions or many smaller seed transactions.
	// We need to check all fields of the transaction, as they can all change the signature.
	transactionIndex := blockHeight - lowestSeedTransactionHeight
	transactionAmount := finalSeedTransactionAmount +
		(initialSeedTransactionAmount-finalSeedTransactionAmount)*
			(totalSeedTransactions-transactionIndex-1)/
			(totalSeedTransactions-1)
	transactionTimestamp := ctxt.BlockAuthority.GetGenesisBlockTimestamp() + blockHeight*configuration.BlockDuration + 1000

	newTransactions := transactions[:0]
	for _, transaction := range transactions {
		if !bytes.Equal(transaction.SenderId, configuration.SeedAccount) {
			newTransactions = append(newTransactions, transaction)
			continue
		}
		if transaction.Type != blockchain_data.TransactionTypeSeed {
			logging.InfoLog.Println("removed non-seed transaction from seed-funding account")
			continue
		}
		if transaction.Amount != transactionAmount {
			logging.InfoLog.Println("removed seed transaction with incorrect amount")
			continue
		}
		if transaction.Timestamp != transactionTimestamp {
			logging.InfoLog.Println("removed seed transaction with incorrect timestamp")
			continue
		}
		if len(transaction.SenderData) > 0 {
			logging.InfoLog.Println("removed seed transaction with non-empty sender data")
			continue
		}
		if transaction.PreviousHashHeight != 0 {
			logging.InfoLog.Println("removed seed transaction with invalid previous hash height")
			continue
		}
		newTransactions = append(newTransactions, transaction)
	}
	return newTransactions
}

func enforceCycleTransactionRules(ctxt *interfaces.Context, transactions []*blockchain_data.Transaction, blockchainVersion int16) []*blockchain_data.Transaction {
	newTransactions := transactions[:0]
	for _, transaction := range transactions {
		if blockchainVersion < 1 && transaction.Type == blockchain_data.TransactionTypeCycle {
			logging.InfoLog.Println("removed cycle transaction due to blockchain version less than 1")
			continue
		}
		if blockchainVersion >= 1 && transaction.Type == blockchain_data.TransactionTypeCycle && !ctxt.CycleAuthority.VerifierInCurrentCycle(transaction.SenderId) {
			logging.InfoLog.Println("removed cycle transaction from out-of-cycle verifier")
			continue
		}
		if blockchainVersion < 2 && transaction.Type == blockchain_data.TransactionTypeCycleSignature {
			logging.InfoLog.Println("removed cycle-signature transaction due to blockchain version less than 2")
			continue
		}
		if blockchainVersion >= 2 && transaction.Type == blockchain_data.TransactionTypeCycleSignature && !ctxt.CycleAuthority.VerifierInCurrentCycle(transaction.SenderId) {
			logging.InfoLog.Println("removed cycle-signature transaction from out-of-cycle verifier")
			continue
		}
		if transaction.Type == blockchain_data.TransactionTypeCycle && transaction.Amount > configuration.MaximumCycleTransactionAmount {
			logging.InfoLog.Println("removed cycle transaction over ∩100,000")
			continue
		}
		if blockchainVersion != 1 && transaction.Type == blockchain_data.TransactionTypeCycle && transaction.CycleSignatures != nil && len(transaction.CycleSignatures) > 0 {
			logging.InfoLog.Println("removed cycle transaction with bundled signatures due to blockchain version not 1")
			continue
		}
		if blockchainVersion == 1 && transaction.Type == blockchain_data.TransactionTypeCycle {
			// Compare signature count to cycle length, don't allow more than one signature per verifier.
			var invalid bool
			observedIds := make([][]byte, 0)
			for _, signature := range transaction.CycleSignatures {
				if !ctxt.CycleAuthority.VerifierInCurrentCycle(signature.Id) || utilities.ByteArrayContains(observedIds, signature.Id) {
					invalid = true
					break
				}
				if !transaction.ValidSignatureBy(signature.Id, signature.Signature) {
					invalid = true
					break
				} else {
					observedIds = append(observedIds, signature.Id)
				}
			}
			if !invalid {
				cycleLength := ctxt.CycleAuthority.GetCurrentCycleLength()
				missingThreshold := cycleLength / 4
				if cycleLength-len(observedIds) > missingThreshold {
					invalid = true
				}
			}
			if invalid {
				logging.InfoLog.Println("removed cycle transaction with invalid bundled signatures")
				continue
			}
		}
		newTransactions = append(newTransactions, transaction)
	}
	return newTransactions
}

func enforceLockingRules(transactions []*blockchain_data.Transaction, blockchainVersion int16, unlockThreshold, unlockTransferSum int64) []*blockchain_data.Transaction {
	if blockchainVersion < 1 {
		return transactions
	}

	// Sum up all transfers from locked accounts.
	var transactionSumFromLockedAccounts int64
	for _, transaction := range transactions {
		if transaction.IsSubjectToLock() {
			transactionSumFromLockedAccounts += transaction.Amount
		}
	}

	// If the sum is greater than the available threshold, remove all transactions subject to locking.
	availableTransferAmount := unlockThreshold - unlockTransferSum
	if transactionSumFromLockedAccounts > availableTransferAmount {
		newTransactions := transactions[:0]
		for _, transaction := range transactions {
			if !transaction.IsSubjectToLock() {
				newTransactions = append(newTransactions, transaction)
			}
		}
		transactions = newTransactions
	}

	return transactions
}

func blockOrderComparator(a, b *blockchain_data.Transaction) bool {
	var byte1, byte2 int
	var result bool
	if a.Timestamp < b.Timestamp {
		return true
	} else if b.Timestamp < a.Timestamp {
		return false
	} else {
		for k := 0; k < len(a.Signature) && k < len(b.Signature); k++ {
			byte1 = int(a.Signature[k]) & 0xff
			byte2 = int(b.Signature[k]) & 0xff
			result = byte1 < byte2
			if byte1 != byte2 {
				break
			}
		}
	}
	return result
}

func amountComparator(a, b *blockchain_data.Transaction) bool {
	amountA := a.Amount
	amountB := b.Amount
	if a.Type == blockchain_data.TransactionTypeCycleSignature {
		amountA = 9223372036854775807
	}
	if b.Type == blockchain_data.TransactionTypeCycleSignature {
		amountB = 9223372036854775807
	}
	return amountA < amountB
}

// Main loop
func (s *state) Start() {
	var lastSeedTransactionCacheHeight int64

	defer logging.InfoLog.Print("Main loop of transaction manager exited gracefully.")
	defer s.ctxt.WaitGroup.Done()
	logging.InfoLog.Print("Starting main loop of transaction manager.")
	done := false
	for !done {
		select {
		case m := <-s.internalMessageChannel:
			switch m.Type {
			case messages.TypeInternalExiting:
				done = true
			case messages.TypeInternalChainInitialized:
				s.chainInitialized = true
			case messages.TypeInternalNewFrozenEdgeBlock:
				block := m.Payload[0].(*blockchain_data.Block)
				s.frozenEdgeHeight = block.Height
				// check to see if we need to load new seed transactions when we freeze the genesis block, then again every 5 blocks (35 seconds)
				// TODO: this can be lengthy, once the transaction manager handles more than just seed transactions, this should run in a goroutine
				if (block.Height == 0 || block.Height-lastSeedTransactionCacheHeight > 4) && block.Height <= highestSeedTransactionHeight && s.chainInitialized {
					lastSeedTransactionCacheHeight = block.Height
					s.cacheSeedTransactions()
				}
			}
		}
	}
}

// Initialization function
func (s *state) Initialize() error {
	// set message routes
	s.internalMessageChannel = make(chan *messages.InternalMessage, 20)
	router.Router.AddInternalRoute(messages.TypeInternalExiting, s.internalMessageChannel)
	router.Router.AddInternalRoute(messages.TypeInternalNewFrozenEdgeBlock, s.internalMessageChannel)
	router.Router.AddInternalRoute(messages.TypeInternalChainInitialized, s.internalMessageChannel)
	s.seedTransactionCache = make(map[int64]*blockchain_data.Transaction)
	// make sure the data directory for seed transactions is there
	err := os.MkdirAll(configuration.DataDirectory+"/"+configuration.SeedTransactionDirectory, os.ModePerm)
	return err
}

// Create a transaction manager.
func NewTransactionManager(ctxt *interfaces.Context) interfaces.TransactionManagerInterface {
	s := &state{}
	s.ctxt = ctxt
	return s
}

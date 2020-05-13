/*
Manage transactions.
*/
package transaction_manager

import (
	"github.com/cryptic-monk/go-nyzo/internal/logging"
	"github.com/cryptic-monk/go-nyzo/internal/nyzo/blockchain_data"
	"github.com/cryptic-monk/go-nyzo/internal/nyzo/configuration"
	"github.com/cryptic-monk/go-nyzo/internal/nyzo/interfaces"
	"github.com/cryptic-monk/go-nyzo/internal/nyzo/messages"
	"github.com/cryptic-monk/go-nyzo/internal/nyzo/messages/message_content/message_fields"
	"github.com/cryptic-monk/go-nyzo/internal/nyzo/router"
	"github.com/cryptic-monk/go-nyzo/internal/nyzo/utilities"
	"os"
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
func (s *state) ValidTransactionsOnly(transactions []*blockchain_data.Transaction, startTimestamp int64) []*blockchain_data.Transaction {
	validTransactions := make([]*blockchain_data.Transaction, 0, 0)
	observedSignatures := make([][]byte, 0, 0)
	endTimestamp := startTimestamp + configuration.BlockDuration
	for _, transaction := range transactions {
		if (transaction.Type == blockchain_data.TransactionTypeCoinGeneration || transaction.SignatureIsValid()) && transaction.Timestamp >= startTimestamp && transaction.Timestamp < endTimestamp {
			var id []byte
			if transaction.Type == blockchain_data.TransactionTypeCoinGeneration {
				id = make([]byte, message_fields.SizeHash, message_fields.SizeHash)
			} else {
				id = transaction.Signature
			}
			if !utilities.ByteArrayContains(observedSignatures, id) {
				observedSignatures = append(observedSignatures, id)
				validTransactions = append(validTransactions, transaction)
			}
		}
	}
	return validTransactions
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

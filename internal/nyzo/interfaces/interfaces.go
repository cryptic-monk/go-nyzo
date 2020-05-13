/*
The interfaces presented here summarize the complex interaction of various components of a node.
*/
package interfaces

import (
	"github.com/cryptic-monk/go-nyzo/internal/nyzo/blockchain_data"
	"github.com/cryptic-monk/go-nyzo/internal/nyzo/networking"
)

// Listen only component.
type Component interface {
	// Initialize component
	Initialize() error
	// Start component/enter main loop if needed
	Start()
}

type BlockFileHandlerInterface interface {
	Component
	// Get a block.
	GetBlock(height int64) *blockchain_data.Block
	// Get multiple blocks in continuous order.
	GetBlocks(heightFrom, heightTo int64) ([]*blockchain_data.Block, error)
	// Load a balance list for the given height, only works if we have an individual block file for that height (for now).
	GetBalanceList(blockHeight int64) *blockchain_data.BalanceList
	// Commit a new frozen edge block. This is blocking and can take a significant amount of time during startup (especially for the archive node).
	CommitFrozenEdgeBlock(block *blockchain_data.Block, balanceList *blockchain_data.BalanceList)
	// Inform the file handler that the chain is fully initialized
	SetChainIsInitialized()
}

type CycleAuthorityInterface interface {
	Component
	GetCurrentCycleLength() int                                                                 // returns the current cycle length
	VerifierInCurrentCycle(id []byte) bool                                                      // returns true if the verifier with this id is currently in the cycle
	GetCycleInformationForBlock(block *blockchain_data.Block) *blockchain_data.CycleInformation // get cycle information for this block
	DetermineContinuityForBlock(block *blockchain_data.Block) int                               // determine this block's continuity (diversity) state
}

type BlockAuthorityInterface interface {
	Component
	// Do a full verification of this block: signature (including transactions) and continuity.
	BlockIsValid(block *blockchain_data.Block) bool
	// Get the genesis block hash, used for seed transactions.
	GetGenesisBlockHash() []byte
}

type TransactionManagerInterface interface {
	Component
	// Returns only valid transactions from the given list, including signature, duplicate and malleability check.
	// Hence the startTimestamp: only transactions between startTimestamp and startTimestamp + BlockDuration are allowed into a block.
	ValidTransactionsOnly(transactions []*blockchain_data.Transaction, startTimestamp int64) []*blockchain_data.Transaction
	SeedTransactionForBlock(height int64) *blockchain_data.Transaction
}

type MeshListenerInterface interface {
	Component
}

type NodeManagerInterface interface {
	Component
	// Are we accepting messages from this peer? Used to quickly terminate a connection.
	AcceptingMessagesFrom(ip string) bool
	// Are we accepting messages of this type from this peer? Used to quickly terminate connections.
	AcceptingMessageTypeFrom(ip string, messageType int16) bool
	// Get a list of trusted entry points.
	GetTrustedEntryPoints() []*networking.TrustedEntryPoint
	// Get a list of managed verifiers.
	GetManagedVerifiers() []*networking.ManagedVerifier
}

type KeyValueStoreInterface interface {
	// Store
	Store(key, value string)
	// Retrieve
	Retrieve(key, defaultValue string) string
}

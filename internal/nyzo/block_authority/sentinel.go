package block_authority

import (
	"bytes"
	"fmt"
	"github.com/cryptic-monk/go-nyzo/internal/logging"
	"github.com/cryptic-monk/go-nyzo/internal/nyzo/balance_authority"
	"github.com/cryptic-monk/go-nyzo/internal/nyzo/blockchain_data"
	"github.com/cryptic-monk/go-nyzo/internal/nyzo/configuration"
	"github.com/cryptic-monk/go-nyzo/internal/nyzo/messages"
	"github.com/cryptic-monk/go-nyzo/internal/nyzo/messages/message_content"
	"github.com/cryptic-monk/go-nyzo/internal/nyzo/messages/message_content/message_fields"
	"github.com/cryptic-monk/go-nyzo/internal/nyzo/networking"
	"github.com/cryptic-monk/go-nyzo/internal/nyzo/router"
	"github.com/cryptic-monk/go-nyzo/internal/nyzo/utilities"
	"strconv"
)

const (
	blockCreationDelay               = 15000 // Java has 20 seconds here, during testing we use 15 seconds to jump in before Java.
	blockTransmissionDelay           = 10000
	minimumBlockTransmissionInterval = 30000
)

type sentinelData struct {
	lastBlockReceivedTime                  int64                    // last time we received a valid block from the mesh
	lastBlockTransmissionHeight            int64                    // height of last block transmission by sentinel
	lastBlockTransmissionTime              int64                    // time of last block transmission by sentinel
	lastBlockTransmissionInfo              string                   // retains some info about last sentinel block transmission
	lastBlockTransmissionResult            string                   // retains some info about last sentinel block transmission success/failure
	blockTransmissionSuccessCount          int                      // number of successful block transmits
	checkBlockTransmission                 bool                     // did we recently transmit a block? if so, check on results after 3 seconds
	calculatingValidChainScores            bool                     // are we able to score the chain?
	blocksForVerifiers                     []*blockchain_data.Block // blocks we prepared for our managed verifiers
	lastNewVerifierBlockTransmissionHeight int64                    // last height at which we broadcast a new verifier block
}

// Create blocks for managed verifiers if necessary, old blocks will be cleared in freezeBlock.
func (s *state) createBlocksForVerifiers() {
	if s.sentinel.blocksForVerifiers == nil {
		s.sentinel.blocksForVerifiers = make([]*blockchain_data.Block, len(s.managedVerifiers))
		for i, managedVerifier := range s.managedVerifiers {
			s.sentinel.blocksForVerifiers[i] = s.createNextBlock(s.frozenEdgeBlock, managedVerifier)
		}
	}
}

// Find lowest scored block among blocks produced for managed verifiers.
func (s *state) findLowestScoredBlockForVerifiers() (lowestScoredBlock *blockchain_data.Block, lowestScore int64) {
	if s.sentinel.blocksForVerifiers == nil {
		return nil, MaxChainScore
	}
	lowestScore = MaxChainScore
	for _, block := range s.sentinel.blocksForVerifiers {
		score := s.chainScore(block, s.frozenEdgeHeight)
		if block != nil && score < lowestScore && s.ctxt.CycleAuthority.VerifierInCurrentCycle(block.VerifierIdentifier) {
			lowestScore = score
			lowestScoredBlock = block
		}
	}
	return lowestScoredBlock, lowestScore
}

// Transmit a block if one of the managed verifiers doesn't do its job.
func (s *state) transmitBlockIfNecessary() {
	// The block creation delay prevents unnecessary work and unnecessary transmissions to the mesh when the
	// sentinel is initializing. We also allow the condition to be entered at least once to confirm that the
	// sentinel is able to calculate valid chain scores.
	if !s.sentinel.calculatingValidChainScores || s.sentinel.lastBlockReceivedTime < utilities.Now()-blockCreationDelay {
		s.createBlocksForVerifiers()
		// for each new height, verify if we should transmit a block
		if s.sentinel.lastBlockTransmissionHeight < s.frozenEdgeHeight+1 {
			lowestScoredBlock, lowestScore := s.findLowestScoredBlockForVerifiers()
			if lowestScore < MaxChainScore-1 {
				s.sentinel.calculatingValidChainScores = true
				// If the block's minimum vote timestamp is in the past, transmit the block now. This is stricter
				// than the verifier, which will transmit a block whose minimum vote timestamp is up to 10 seconds
				// in the future.
				minimumVoteTimestamp := s.frozenEdgeBlock.VerificationTimestamp + configuration.MinimumVerificationInterval + lowestScore*20000 + blockTransmissionDelay
				now := utilities.Now()
				if minimumVoteTimestamp < now && s.sentinel.lastBlockTransmissionTime < now-minimumBlockTransmissionInterval {
					s.sentinel.lastBlockTransmissionTime = now
					verifier := s.findManagedVerifierById(lowestScoredBlock.VerifierIdentifier)
					content := message_content.NewNewBlock(lowestScoredBlock)
					message := messages.NewLocal(messages.TypeNewBlock, content, verifier.Identity)
					router.Router.RouteInternal(messages.NewInternalMessage(messages.TypeInternalSendToCycle, message))
					s.sentinel.lastBlockTransmissionHeight = lowestScoredBlock.Height
					s.sentinel.lastBlockTransmissionInfo = lowestScoredBlock.String()
					s.sentinel.blockTransmissionSuccessCount = 0
					s.sentinel.checkBlockTransmission = true
					s.ctxt.PersistentData.Store(configuration.LastBlockTransmissionHeightKey, strconv.FormatInt(s.sentinel.lastBlockTransmissionHeight, 10))
					s.ctxt.PersistentData.Store(configuration.LastBlockTransmissionInfoKey, s.sentinel.lastBlockTransmissionInfo)
					logging.InfoLog.Printf("sent block for %v with hash %v at height %v",
						utilities.ByteArrayToString(lowestScoredBlock.VerifierIdentifier),
						utilities.ByteArrayToString(lowestScoredBlock.Hash),
						lowestScoredBlock.Height)
				}
			}
		}
	}
}

// Check on sentinel block transmission results.
func (s *state) checkBlockTransmissionResult() {
	if s.sentinel.checkBlockTransmission && utilities.Now()-s.sentinel.lastBlockTransmissionTime > 3000 {
		s.sentinel.checkBlockTransmission = false
		failures := s.ctxt.CycleAuthority.GetCurrentCycleLength() - s.sentinel.blockTransmissionSuccessCount
		s.sentinel.lastBlockTransmissionResult = fmt.Sprintf("%v success, %v fail", s.sentinel.blockTransmissionSuccessCount, failures)
		s.ctxt.PersistentData.Store(configuration.LastBlockTransmissionResultKey, s.sentinel.lastBlockTransmissionResult)
		logging.InfoLog.Print("transmission results: " + s.sentinel.lastBlockTransmissionResult)
	}
}

// Find a verifier in the managed verifiers list.
func (s *state) findManagedVerifierById(id []byte) *networking.ManagedVerifier {
	for _, verifier := range s.managedVerifiers {
		if bytes.Equal(id, verifier.Identity.PublicKey) {
			return verifier
		}
	}
	return nil
}

// Load persistent data related to sentinel behavior.
func (s *state) loadSentinelPersistentData() {
	s.sentinel.lastBlockTransmissionHeight, _ = strconv.ParseInt(s.ctxt.PersistentData.Retrieve(configuration.LastBlockTransmissionHeightKey, "0"), 10, 64)
	s.sentinel.lastBlockTransmissionInfo = s.ctxt.PersistentData.Retrieve(configuration.LastBlockTransmissionInfoKey, "")
	s.sentinel.lastBlockTransmissionResult = s.ctxt.PersistentData.Retrieve(configuration.LastBlockTransmissionResultKey, "")
}

// Sentinel style block creation.
// TODO: sentinel transaction not implemented yet (not needed for consensus to work).
func (s *state) createNextBlock(previousBlock *blockchain_data.Block, managedVerifier *networking.ManagedVerifier) *blockchain_data.Block {
	var block *blockchain_data.Block
	if previousBlock != nil && !bytes.Equal(previousBlock.VerifierIdentifier, managedVerifier.Identity.PublicKey) {
		block = &blockchain_data.Block{
			BlockchainVersion:     previousBlock.BlockchainVersion,
			Height:                previousBlock.Height + 1,
			PreviousBlockHash:     previousBlock.Hash,
			StartTimestamp:        s.ctxt.BlockAuthority.GetGenesisBlockTimestamp() + (previousBlock.Height+1)*configuration.BlockDuration,
			VerificationTimestamp: utilities.Now(),
			VerifierIdentifier:    managedVerifier.Identity.PublicKey,
		}
		transactions := s.ctxt.TransactionManager.TransactionsForHeight(block.Height)
		seedTransaction := s.ctxt.TransactionManager.SeedTransactionForBlock(block.Height)
		if seedTransaction != nil {
			transactions = append(transactions, seedTransaction)
		}
		sentinelTransaction := s.getSentinelTransaction(previousBlock, managedVerifier)
		if sentinelTransaction != nil {
			transactions = append(transactions, sentinelTransaction)
		}
		transactions = s.ctxt.TransactionManager.ApprovedTransactionsForBlock(transactions, previousBlock, true)
		block.Transactions = transactions
		balanceList := s.ctxt.BlockHandler.GetBalanceListForBlock(previousBlock)
		if balanceList == nil {
			return nil
		}
		balanceList = balance_authority.UpdateBalanceListForNextBlock(s.ctxt, previousBlock.VerifierIdentifier, balanceList, block, false)
		if balanceList == nil {
			return nil
		}
		block.BalanceListHash = balanceList.GetHash()
		block.VerifierSignature = managedVerifier.Identity.Sign(block.Serialize(true))
		doubleSha := utilities.DoubleSha256(block.VerifierSignature)
		block.Hash = doubleSha[:]
	}
	return block
}

func (s *state) getSentinelTransaction(previousBlock *blockchain_data.Block, managedVerifier *networking.ManagedVerifier) *blockchain_data.Transaction {
	// If the verifier is supposed to add a sentinel transaction, add it now. This is a 1-micronyzo transaction
	// from the sender to a dead wallet (all zeros). The minimum transaction fee is 1 micronyzo, so no funds
	// will be transferred. The only purpose of this transaction is to add metadata to the blockchain.
	// Out-of-cycle verifiers do not add sentinel transactions, as these transactions would complicate the
	// block-rebuilding process.
	if managedVerifier.SentinelTransactionEnabled && s.ctxt.CycleAuthority.VerifierInCurrentCycle(managedVerifier.Identity.PublicKey) {
		balanceList := s.ctxt.BlockHandler.GetBalanceListForBlock(previousBlock)
		if balanceList == nil {
			logging.InfoLog.Print("omitting sentinel transaction due to unavailable balance list")
		} else {
			// Only add the sentinel transaction if the balance is over the minimum preferred balance.
			verifierBalance := balanceList.GetBalance(managedVerifier.Identity.PublicKey)
			if verifierBalance <= configuration.MinimumPreferredBalance {
				logging.InfoLog.Printf("omitting sentinel transaction because balance of %v is %v, which is less than minimum preferred balance of %v",
					utilities.ByteArrayToString(managedVerifier.Identity.PublicKey),
					verifierBalance,
					configuration.MinimumPreferredBalance)
			} else {
				transaction := &blockchain_data.Transaction{
					Type:               blockchain_data.TransactionTypeStandard,
					Timestamp:          previousBlock.StartTimestamp + configuration.BlockDuration + 1,
					Amount:             1,
					RecipientId:        make([]byte, message_fields.SizeNodeIdentifier, message_fields.SizeNodeIdentifier),
					PreviousHashHeight: previousBlock.Height,
					PreviousBlockHash:  previousBlock.Hash,
					SenderId:           managedVerifier.Identity.PublicKey,
					SenderData:         []byte("sentinel version " + configuration.Version),
				}
				transaction.Signature = managedVerifier.Identity.Sign(transaction.Serialize(true))
				return transaction
			}
		}
	}
	return nil
}

// Periodically send out minimal blocks for verifiers with potential to join the cycle.
func (s *state) broadcastNewVerifierBlock() {
	height := s.frozenEdgeHeight + 1
	if height%50 == 49 &&
		height > s.sentinel.lastNewVerifierBlockTransmissionHeight &&
		height <= s.ctxt.BlockAuthority.GetOpenEdgeHeight(false) &&
		s.likelyAcceptingNewVerifiers() {
		s.sentinel.lastNewVerifierBlockTransmissionHeight = height
		for _, verifier := range s.managedVerifiers {
			if !s.ctxt.CycleAuthority.VerifierInCurrentCycle(verifier.Identity.PublicKey) {
				logging.InfoLog.Printf("sending UDP block for %v at height %v", utilities.ByteArrayToString(verifier.Identity.PublicKey), height)
				s.broadcastUdpBlockForNewVerifier(verifier)
			}
		}
	}
}

func (s *state) broadcastUdpBlockForNewVerifier(verifier *networking.ManagedVerifier) {
	block := s.createNextBlock(s.frozenEdgeBlock, verifier)
	content := message_content.NewMinimalBlock(block.VerificationTimestamp, block.VerifierSignature)
	message := messages.NewLocal(messages.TypeMinimalBlock, content, verifier.Identity)
	router.Router.RouteInternal(messages.NewInternalMessage(messages.TypeInternalSendToCycleUdp, message))
}

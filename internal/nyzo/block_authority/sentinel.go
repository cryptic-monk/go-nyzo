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
	"github.com/cryptic-monk/go-nyzo/internal/nyzo/networking"
	"github.com/cryptic-monk/go-nyzo/internal/nyzo/router"
	"github.com/cryptic-monk/go-nyzo/internal/nyzo/utilities"
	"strconv"
	"time"
)

const (
	blockCreationDelay               = 15000 // Java has 20 seconds here, during testing we use 15 seconds to jump in before Java.
	blockTransmissionDelay           = 10000
	minimumBlockTransmissionInterval = 30000
)

type sentinelData struct {
	lastBlockReceivedTime          int64  // last time we received a valid block from the mesh
	lastBlockTransmissionHeight    int64  // height of last block transmission by sentinel
	lastBlockTransmissionTimestamp int64  // timestamp of last block transmission by sentinel
	lastBlockTransmissionInfo      string // retains some info about last sentinel block transmission
	lastBlockTransmissionResult    string // retains some info about last sentinel block transmission success/failure
	blockTransmissionSuccessCount  int    // number of successful block transmits
	checkBlockTransmission         bool
	calculatingValidChainScores    bool                     // are we able to score the chain?
	blocksForVerifiers             []*blockchain_data.Block // blocks we produced for our managed verifiers
}

func (s *state) transmitBlockIfNecessary() {
	// The block creation delay prevents unnecessary work and unnecessary transmissions to the mesh when the
	// sentinel is initializing. We also allow the condition to be entered at least once to confirm that the
	// sentinel is able to calculate valid chain scores.
	if !s.sentinel.calculatingValidChainScores || s.sentinel.lastBlockReceivedTime < (time.Now().UnixNano()/1000000)-blockCreationDelay {
		// create blocks if necessary, old blocks will be cleared in freezeBlock
		if s.sentinel.blocksForVerifiers == nil {
			s.sentinel.blocksForVerifiers = make([]*blockchain_data.Block, len(s.managedVerifiers))
			for i, managedVerifier := range s.managedVerifiers {
				s.sentinel.blocksForVerifiers[i] = s.createNextBlock(s.frozenEdgeBlock, managedVerifier)
			}
		}
		// for each new height, verify if we should transmit a block
		if s.sentinel.lastBlockTransmissionHeight < s.frozenEdgeHeight+1 {
			// find lowest scored block
			var lowestScoredBlock *blockchain_data.Block
			var lowestScore int64 = MaxChainScore
			for _, block := range s.sentinel.blocksForVerifiers {
				score := s.chainScore(block, s.frozenEdgeHeight)
				if block != nil && score < lowestScore && s.ctxt.CycleAuthority.VerifierInCurrentCycle(block.VerifierIdentifier) {
					lowestScore = score
					lowestScoredBlock = block
				}
			}
			if lowestScore < MaxChainScore-1 && lowestScoredBlock != nil {
				s.sentinel.calculatingValidChainScores = true
				// If the block's minimum vote timestamp is in the past, transmit the block now. This is stricter
				// than the verifier, which will transmit a block whose minimum vote timestamp is up to 10 seconds
				// in the future.
				minimumVoteTimestamp := s.frozenEdgeBlock.VerificationTimestamp + configuration.MinimumVerificationInterval + lowestScore*20000 + blockTransmissionDelay
				now := time.Now().UnixNano() / 1000000
				if minimumVoteTimestamp < now && s.sentinel.lastBlockTransmissionTimestamp < now-minimumBlockTransmissionInterval {
					s.sentinel.lastBlockTransmissionTimestamp = now
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
	if s.sentinel.checkBlockTransmission && time.Now().UnixNano()/1000000-s.sentinel.lastBlockTransmissionTimestamp > 3000 {
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
			VerificationTimestamp: time.Now().UnixNano() / 1000000,
			Transactions:          nil,
			BalanceListHash:       nil,
			VerifierIdentifier:    managedVerifier.Identity.PublicKey,
			VerifierSignature:     nil,
			ContinuityState:       0,
			SignatureState:        0,
			CycleInformation:      nil,
			Hash:                  nil,
			CycleInfoCache:        nil,
		}
		transactions := s.ctxt.TransactionManager.TransactionsForHeight(block.Height)
		seedTransaction := s.ctxt.TransactionManager.SeedTransactionForBlock(block.Height)
		if seedTransaction != nil {
			transactions = append(transactions, seedTransaction)
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

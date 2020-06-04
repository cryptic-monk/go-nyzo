package block_authority

import (
	"bytes"
	"github.com/cryptic-monk/go-nyzo/internal/nyzo/blockchain_data"
	"github.com/cryptic-monk/go-nyzo/internal/nyzo/networking"
	"time"
)

const blockCreationDelay = 20000

func (s *state) transmitBlockIfNecessary() {
	if !s.calculatingValidChainScores || s.lastBlockReceivedTime < (time.Now().UnixNano()/1000000)-blockCreationDelay {
		// create blocks if necessary
		if s.blocksForVerifiers == nil {
			s.blocksForVerifiers = make([]*blockchain_data.Block, len(s.managedVerifiers))
			for i, managedVerifier := range s.managedVerifiers {
				block := s.createNextBlock(s.frozenEdgeBlock, managedVerifier)
				s.blocksForVerifiers[i] = block
			}
		}
	}
}

func (s *state) createNextBlock(previousBlock *blockchain_data.Block, managedVerifier *networking.ManagedVerifier) *blockchain_data.Block {
	var block *blockchain_data.Block
	if previousBlock != nil && !bytes.Equal(previousBlock.VerifierIdentifier, managedVerifier.Identity.PublicKey) {
		blockHeight := previousBlock.Height + 1
		transactions := s.ctxt.TransactionManager.TransactionsForHeight(blockHeight)
		seedTransaction := s.ctxt.TransactionManager.SeedTransactionForBlock(blockHeight)
		if seedTransaction != nil {
			transactions = append(transactions, seedTransaction)
		}
	}
	return block
}

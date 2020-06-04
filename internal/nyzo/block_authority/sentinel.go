package block_authority

import (
	"bytes"
	"github.com/cryptic-monk/go-nyzo/internal/nyzo/balance_authority"
	"github.com/cryptic-monk/go-nyzo/internal/nyzo/blockchain_data"
	"github.com/cryptic-monk/go-nyzo/internal/nyzo/configuration"
	"github.com/cryptic-monk/go-nyzo/internal/nyzo/networking"
	"github.com/cryptic-monk/go-nyzo/internal/nyzo/utilities"
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
		block = &blockchain_data.Block{
			BlockchainVersion:     previousBlock.BlockchainVersion,
			Height:                previousBlock.Height + 1,
			PreviousBlockHash:     previousBlock.Hash,
			StartTimestamp:        s.ctxt.BlockAuthority.GetGenesisBlockTimestamp() + block.Height*configuration.BlockDuration,
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

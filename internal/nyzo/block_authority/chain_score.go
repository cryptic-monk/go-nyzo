package block_authority

import (
	"bytes"
	"github.com/cryptic-monk/go-nyzo/internal/nyzo/balance_authority"
	"github.com/cryptic-monk/go-nyzo/internal/nyzo/blockchain_data"
	"github.com/cryptic-monk/go-nyzo/internal/nyzo/configuration"
	"github.com/cryptic-monk/go-nyzo/internal/nyzo/interfaces"
	"github.com/cryptic-monk/go-nyzo/internal/nyzo/utilities"
)

const (
	MaxChainScore = 9223372036854775807
)

// Scores the chain tip at the given block, down to zeroBlockHeight + 1. The higher the score,
// the worse we consider this version of the chain. MaxChainScore: an invalid chain,
// MaxChainScore - 1: a chain that could be salvageable with more info.
func (s *state) chainScore(block *blockchain_data.Block, zeroBlockHeight int64) int64 {
	var score int64
	topNewVerifier := s.ctxt.CycleAuthority.GetTopNewVerifier()
	for block != nil && block.Height > zeroBlockHeight && score < MaxChainScore-1 {
		cycleInformation := s.ctxt.CycleAuthority.GetCycleInformationForBlock(block)
		continuityState := s.ctxt.CycleAuthority.DetermineContinuityForBlock(block)
		if bytes.Equal(s.ctxt.Identity.PublicKey, block.VerifierIdentifier) &&
			!s.ctxt.CycleAuthority.VerifierInCurrentCycle(s.ctxt.Identity.PublicKey) &&
			bytes.Equal(s.ctxt.Identity.PublicKey, topNewVerifier) {
			// This is likely an incorrect score. If a different verifier has joined recently, then the correct
			// score would be Long.MAX_VALUE to signify a discontinuity. However, the only consequence of the
			// incorrect score would be transmission of an invalid block, which would quickly be rejected by the
			// entire cycle. As this verifier is out-of-cycle, it does not yet have any voting power, so this
			// miscalculation does not weaken the system in any way.
			score = -2
		} else if cycleInformation == nil || continuityState == blockchain_data.Undetermined {
			score = MaxChainScore - 1
		} else {
			if continuityState == blockchain_data.Invalid {
				score = MaxChainScore
			} else if cycleInformation.NewVerifier {
				if cycleInformation.InGenesisCycle {
					// This is a special case for the Genesis cycle. We want a deterministic order that can be
					// calculated locally, but we do not care what that order is.
					score = (utilities.LongSha256(block.VerifierIdentifier)%9000)*-1 - 1000
				} else {
					// Only allow the top new verifier to join.
					if bytes.Equal(block.VerifierIdentifier, topNewVerifier) {
						score -= 2
					} else {
						score += 10000
					}
					// Penalize for each balance-list spam transaction.
					score += int64(spamTransactionCount(s.ctxt, block)) * 5
					// Penalize a smaller amount for each excess transaction. Excess transactions have no lingering
					// negative effects, but including them in the scoring does provide an assurance of an upper
					// limit.
					score += int64(excessTransactionCount(s.ctxt, block) / 10)
				}
			} else {
				previousBlock := s.ctxt.BlockHandler.GetBlock(block.Height-1, block.PreviousBlockHash)
				if previousBlock == nil || s.ctxt.CycleAuthority.GetCycleInformationForBlock(previousBlock) == nil {
					score = MaxChainScore - 1
				} else {
					score += (previousBlock.CycleInformation.GetCycleLength() - cycleInformation.GetCycleLength()) * 4
					// If a verifier needs to be removed, apply a penalty score of 5. This will put it just behind
					// the next verifier in the cycle.
					if score == 0 && s.ctxt.CycleAuthority.ShouldPenalizeVerifier(block.VerifierIdentifier) {
						score += 5
					}
					// Penalize for each balance-list spam transaction.
					score += int64(spamTransactionCount(s.ctxt, block)) * 5
					// Penalize a smaller amount for each excess transaction. Excess transactions have no lingering
					// negative effects, but including them in the scoring does provide an assurance of an upper
					// limit.
					score += int64(excessTransactionCount(s.ctxt, block)) / 10
					// Account for the blockchain version. If this is a version downgrade, a skip in versions, or
					// higher than the maximum allowed version, the block is invalid.
					if block.BlockchainVersion < previousBlock.BlockchainVersion ||
						block.BlockchainVersion > previousBlock.BlockchainVersion+1 ||
						block.BlockchainVersion > configuration.MaximumBlockchainVersion {
						score = MaxChainScore
					} else if configuration.IsMissedUpgradeOpportunity(block.Height, block.BlockchainVersion, previousBlock.BlockchainVersion) {
						// In this case, an upgrade is allowed but this block is not an upgrade. Apply a 1-point
						// penalty to this block to encourage an upgrade block from the same verifier to be approved
						// if available.
						score += 1
					} else if configuration.IsImproperlyTimedUpgrade(block.Height, block.BlockchainVersion, previousBlock.BlockchainVersion) {
						// In this case, the block is an upgrade when we are not looking to upgrade. This is a valid
						// block, but it is not preferred. Apply a large penalty.
						score += 10000
					}
				}
			}
		}
		// Check the verification timestamp interval.
		previousBlock := s.ctxt.BlockHandler.GetBlock(block.Height-1, block.PreviousBlockHash)
		if previousBlock != nil && previousBlock.VerificationTimestamp > block.VerificationTimestamp-configuration.MinimumVerificationInterval {
			score = MaxChainScore // invalid
		}
		// Check that the verification timestamp is not unreasonably far into the future.
		if block.VerificationTimestamp > utilities.Now()+5000 {
			score = MaxChainScore // invalid
		}
		block = previousBlock
	}
	if block == nil {
		score = MaxChainScore - 1
	}
	return score
}

// Assess the number of spam transactions in the given block.
func spamTransactionCount(ctxt *interfaces.Context, block *blockchain_data.Block) int {
	var count int
	previousBlock := ctxt.BlockHandler.GetBlock(block.Height-1, block.PreviousBlockHash)
	if previousBlock != nil {
		balanceList := ctxt.BlockHandler.GetBalanceListForBlock(previousBlock)
		if balanceList != nil {
			count = balance_authority.NumberOfTransactionsSpammingBalanceList(balanceList, block.Transactions)
		}
	}
	return count
}

// Too many transactions in this block?
func excessTransactionCount(ctxt *interfaces.Context, block *blockchain_data.Block) int {
	excess := len(block.Transactions) - ctxt.CycleAuthority.GetMaximumTransactionsForBlockAssembly()
	if excess > 0 {
		excess = 0
	}
	return excess
}

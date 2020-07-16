package block_authority

import (
	"github.com/cryptic-monk/go-nyzo/internal/nyzo/blockchain_data"
	"github.com/cryptic-monk/go-nyzo/internal/nyzo/interfaces"
	"github.com/cryptic-monk/go-nyzo/internal/nyzo/utilities"
)

type Report struct {
	RunMode             string                 `json:"run_mode"`
	ProtectingVerifiers string                 `json:"protecting_verifiers"`
	FrozenEdge          int64                  `json:"frozen_edge"`
	OpenEdge            int64                  `json:"open_edge"`
	LowestScoredBlock   *blockchain_data.Block `json:"lowest_scored_block"`
	LowestScore         int64                  `json:"lowest_score"`
}

// Get a status report for this component.
// Concurrency: handled via state access lock.
func (s *state) GetStatusReport() interface{} {
	s.m.Lock()
	defer s.m.Unlock()
	r := Report{}
	r.RunMode = interfaces.GetRunModeId(s.ctxt.RunMode())
	r.FrozenEdge = s.frozenEdgeHeight
	r.OpenEdge = s.GetOpenEdgeHeight(false)
	if s.ctxt.RunMode() == interfaces.RunModeSentinel || s.ctxt.RunMode() == interfaces.RunModeArchive {
		if !s.sentinel.calculatingValidChainScores {
			r.ProtectingVerifiers = "no"
		} else if s.frozenEdgeBlock == nil || s.frozenEdgeBlock.VerificationTimestamp < utilities.Now()-80000 {
			r.ProtectingVerifiers = "uncertain"
		} else {
			r.ProtectingVerifiers = "yes"
		}
		s.createBlocksForVerifiers()
		lowestScoredBlock, lowestScore := s.findLowestScoredBlockForVerifiers()
		r.LowestScoredBlock = lowestScoredBlock
		r.LowestScore = lowestScore
	}
	return r
}

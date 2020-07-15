/*
Used to attach additional cycle info to a block, this is not part of the original spec, it exists to speed up
cycle-related calculations and is only of local relevance.
*/
package blockchain_data

type BlockCycleInfoCache struct {
	Found           bool  `json:"found"`
	IsGenesis       bool  `json:"is_genesis"`
	NewVerifier     bool  `json:"new_verifier"`
	CycleTailHeight int64 `json:"cycle_tail_height"`
	CycleHeadHeight int64 `json:"cycle_head_height"`
}

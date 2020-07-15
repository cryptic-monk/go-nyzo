/*
Gets attached to a block, represents info about the current and past cycles.
*/
package blockchain_data

type CycleInformation struct {
	MaximumCycleLength int   `json:"maximum_cycle_length"`
	CycleLengths       []int `json:"cycle_lengths"`
	NewVerifier        bool  `json:"new_verifier"`
	InGenesisCycle     bool  `json:"in_genesis_cycle"`
}

// The trailing edge is 4 cycles back from the current height (or 0)
func (c CycleInformation) CalculateTrailingEdgeHeight(blockHeight int64) int64 {
	height := blockHeight - int64(c.CycleLengths[0]) - int64(c.CycleLengths[1]) - int64(c.CycleLengths[2]) - int64(c.CycleLengths[3])
	if height < 0 {
		height = 0
	}
	return height
}

// Convenience to get the current cycle length.
func (c CycleInformation) GetCycleLength() int64 {
	return int64(c.CycleLengths[0])
}

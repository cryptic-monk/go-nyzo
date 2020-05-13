/*
Gets attached to a block, represents info about the current and past cycles.
*/
package blockchain_data

type CycleInformation struct {
	MaximumCycleLength int
	CycleLengths       []int
	NewVerifier        bool
	InGenesisCycle     bool
}

// The trailing edge is 4 cycles back from the current height (or 0)
func (c CycleInformation) CalculateTrailingEdgeHeight(blockHeight int64) int64 {
	height := blockHeight - int64(c.CycleLengths[0]) - int64(c.CycleLengths[1]) - int64(c.CycleLengths[2]) - int64(c.CycleLengths[3])
	if height < 0 {
		height = 0
	}
	return height
}

/*
Used to attach additional cycle info to a block, this is not part of the original spec, it exists to speed up
cycle-related calculations and is only of local relevance.
*/
package blockchain_data

type BlockCycleInfoCache struct {
	Found, IsGenesis, NewVerifier bool
	CycleTailHeight               int64
	CycleHeadHeight               int64
}

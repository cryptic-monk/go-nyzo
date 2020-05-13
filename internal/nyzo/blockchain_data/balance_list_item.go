/*
Individual entry in a balance list.
*/
package blockchain_data

type BalanceListItem struct {
	Identifier     []byte
	Balance        int64
	BlocksUntilFee int16
}

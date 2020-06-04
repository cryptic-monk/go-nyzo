package transaction_manager

import "github.com/cryptic-monk/go-nyzo/internal/nyzo/blockchain_data"

func (s *state) TransactionsForHeight(height int64) []*blockchain_data.Transaction {
	return make([]*blockchain_data.Transaction, 0)
}

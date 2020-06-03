package block_handler

import "github.com/cryptic-monk/go-nyzo/internal/nyzo/blockchain_data"

// Get a yet to be frozen block. Since multiple candidate blocks can exist at a certain height, a block
// hash is needed.
func (s *state) GetUnfrozenBlock(height int64, hash []byte) *blockchain_data.Block {
	return nil
}

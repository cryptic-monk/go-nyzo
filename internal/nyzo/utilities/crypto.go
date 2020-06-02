/*
Simple crypto utilities.
*/
package utilities

import (
	"crypto/sha256"
	"encoding/binary"
)

func DoubleSha256(input []byte) [32]byte {
	singleSha256 := sha256.Sum256(input)
	return sha256.Sum256(singleSha256[:])
}

// Helper to calculate a long from a byte array (used for genesis cycle serialization).
func LongSha256(input []byte) int64 {
	singleSha256 := sha256.Sum256(input)
	return int64(binary.BigEndian.Uint64(singleSha256[0:8]))
}

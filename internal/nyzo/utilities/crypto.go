/*
Simple crypto utilities.
*/
package utilities

import "crypto/sha256"

func DoubleSha256(input []byte) [32]byte {
	singleSha256 := sha256.Sum256(input)
	return sha256.Sum256(singleSha256[:])
}

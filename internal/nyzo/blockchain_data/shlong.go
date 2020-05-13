/*
A special data type sometimes used to save a little space during data serialization.
*/
package blockchain_data

// Historical comment from Java version:
// This class provides a backward-compatible version/height combination that does not change the beginning bytes of
// blocks or balance lists. The layout is as follows:
// (1) 2 bytes: version [0-0xFFFF]
// (2) 6 bytes: block height [0-0xFFFFFFFFFFFF]
// In decimal notation, this provides version numbers from 0 through 65535 and block heights from 0 through
// 281,474,976,710,655. This allows for over 60 million years of blocks.

// Takes a shlong and turns it into a long and a short.
func FromShlong(combined int64) (int16, int64) {
	return int16((combined >> 48) & 0xffff), combined & 0xffffffffffff
}

// Takes a long and a short and turns it into a shlong.
func ToShlong(short int16, long int64) int64 {
	return int64(short)<<48 | long
}

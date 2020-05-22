/*
Some helper functions used in various parts of the system.
*/
package utilities

import (
	"bytes"
)

// Returns true if the given array of byte arrays contains lookFor
func ByteArrayContains(array [][]byte, lookFor []byte) bool {
	for _, item := range array {
		if bytes.Equal(item, lookFor) {
			return true
		}
	}
	return false
}

// Compares two byte arrays for sorting purposes.
// Return: true if a should come first in ascending order, false otherwise.
func ByteArrayComparator(a, b []byte) bool {
	var byte1, byte2 int
	result := false
	for k := 0; k < len(a) && k < len(b); k++ {
		byte1 = int(a[k]) & 0xff
		byte2 = int(b[k]) & 0xff
		result = byte1 < byte2
		if byte1 != byte2 {
			break
		}
	}
	return result
}

// Make a clean copy of the <source> byte array with the given length to reduce memory footprint.
func ByteArrayCopy(source []byte, length int) []byte {
	result := make([]byte, length)
	copy(result, source)
	return result
}

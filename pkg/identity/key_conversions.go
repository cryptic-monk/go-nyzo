package identity

import (
	"encoding/hex"
	"errors"
	"fmt"
	"github.com/cryptic-monk/go-nyzo/internal/nyzo/utilities"
)

const (
	NyzoStringTypePrivateKey   = 1
	NyzoStringPrivateKeyPrefix = "key_"
	NyzoStringTypePublicKey    = 2
	NyzoStringPublicKeyPrefix  = "id__"
	NyzoStringCharacters       = "0123456789abcdefghijkmnopqrstuvwxyzABCDEFGHIJKLMNPQRSTUVWXYZ-.~_"
)

var NyzoStringCharacterMap map[byte]int

// NyzoHexToBytes converts 'data' - a sequence of hex character bytes - to a byte array of the defined length.
// So, [102 54] (aka 'f6') will turn into [246].
// The function mimics the somewhat idiosyncratic behavior of the original java function by ignoring any non-hex
// characters in the input data. Like that, separator dashes ('-') will be filtered out, but so would any other
// non-hex characters. Also like the original function, we ignore if we get too much input data for the desired
// result length. We do however NOT ignore if we don't get enough input data, so unless there is an error returned,
// the result will always have the desired length.
func NyzoHexToBytes(data []byte, length int) ([]byte, error) {
	cleaned := make([]byte, length*2)
	index := 0
	for _, character := range data {
		// 0 .. 9, A .. F, a .. f
		if (character >= 48 && character <= 57) || (character >= 65 && character <= 70) || (character >= 97 && character <= 102) {
			cleaned[index] = character
			index++
			if index == length*2 {
				break
			}
		}
	}
	if index != length*2 {
		message := fmt.Sprintf("Cannot convert hex characters to bytes, not enough characters in source data")
		return nil, errors.New(message)
	}
	decoded := make([]byte, length)
	_, err := hex.Decode(decoded, cleaned)
	if err != nil {
		message := fmt.Sprintf("Cannot convert hex characters to bytes: %s", err.Error())
		return nil, errors.New(message)
	}
	return decoded, nil
}

// BytesToNyzoHex converts the given byte array to a Nyzo style hex string with separating dashes.
func BytesToNyzoHex(data []byte) string {
	withoutDashes := hex.EncodeToString(data)
	withDashes := ""
	for i := 0; i < len(withoutDashes); i++ {
		if i != 0 && i%16 == 0 {
			withDashes += "-"
		}
		withDashes += withoutDashes[i : i+1]
	}
	return withDashes
}

// ToNyzoString converts the given content data to a Nyzo string of the given type.
// A Nyzo string consists of: prefix + content length + content + checksum.
// In terms of its underlying byte array, the prefix is always 3 bytes long, the content length is
// expressed in one byte, then comes the content, and finally, the checksum is a minimum of 4 bytes and a maximum of
// 6 bytes, widening the expanded array so that its length is always divisible by 3.
// This byte arrray is then mapped to a select set of characters for human readability, resulting in what is commonly
// known as a "Nyzo String".
// TODO: only public and private key string types are supported, add the others: micropay, prefilled and transactions.
func ToNyzoString(stringType int, content []byte) string {
	var prefix []byte
	switch stringType {
	case NyzoStringTypePrivateKey:
		prefix = encodedStringToBytes(NyzoStringPrivateKeyPrefix)
	case NyzoStringTypePublicKey:
		prefix = encodedStringToBytes(NyzoStringPublicKeyPrefix)
	default:
		prefix = []byte{0, 0, 0}
	}
	checksumLength := 4 + (3-(len(content)+2)%3)%3
	var allBytes []byte
	allBytes = append(allBytes, prefix...)
	allBytes = append(allBytes, byte(len(content)))
	allBytes = append(allBytes, content...)
	checksum := utilities.DoubleSha256(allBytes)
	allBytes = append(allBytes, checksum[0:checksumLength]...)
	return bytesToEncodedString(allBytes)
}

// encodedStringToBytes converts a Nyzo string style encoded string to a byte array
func encodedStringToBytes(encodedString string) []byte {
	length := (len(encodedString)*6 + 7) / 8
	array := make([]byte, length)
	for i := 0; i < length; i++ {
		left := encodedString[i*8/6]
		right := encodedString[i*8/6+1]
		leftValue, ok := NyzoStringCharacterMap[left]
		if !ok {
			leftValue = 0
		}
		rightValue, ok := NyzoStringCharacterMap[right]
		if !ok {
			rightValue = 0
		}
		bitOffset := uint32((i * 2) % 6)
		array[i] = byte((((uint32(leftValue) << 6) + uint32(rightValue)) >> (4 - bitOffset)) & 0xff)
	}
	return array
}

// bytesToEncodedString converts a byte array to a Nyzo string style encoded string
func bytesToEncodedString(array []byte) string {
	index := 0
	bitOffset := 0
	encodedString := ""
	for index < len(array) {
		left := uint32(array[index] & 0xff)
		right := uint32(0)
		if index < len(array)-1 {
			right = uint32(array[index+1] & 0xff)
		}
		lookupIndex := (((left << 8) + right) >> (10 - bitOffset)) & 0x3f
		encodedString += NyzoStringCharacters[lookupIndex : lookupIndex+1]
		if bitOffset == 0 {
			bitOffset = 6
		} else {
			index++
			bitOffset -= 2
		}
	}
	return encodedString
}

// set up the character map for Nyzo string encoding
func init() {
	NyzoStringCharacterMap = make(map[byte]int)
	for i := 0; i < len(NyzoStringCharacters); i++ {
		NyzoStringCharacterMap[NyzoStringCharacters[i]] = i
	}
}

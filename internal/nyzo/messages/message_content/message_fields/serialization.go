/*
Byte level message field serialization.
*/
package message_fields

import (
	"encoding/binary"
	"net"
	"os"
)

func SerializeInt16(number int16) []byte {
	buffer := make([]byte, 2)
	binary.BigEndian.PutUint16(buffer, uint16(number))
	return buffer
}

func DeserializeInt16(bytes []byte) int16 {
	return int16(binary.BigEndian.Uint16(bytes))
}

func Int16FromFile(f *os.File) (int16, error) {
	b := make([]byte, 2)
	_, err := f.Read(b)
	if err != nil {
		return 0, err
	}
	return DeserializeInt16(b), nil
}

func SerializeInt32(number int32) []byte {
	buffer := make([]byte, 4)
	binary.BigEndian.PutUint32(buffer, uint32(number))
	return buffer
}

func DeserializeInt32(bytes []byte) int32 {
	return int32(binary.BigEndian.Uint32(bytes))
}

func Int32FromFile(f *os.File) (int32, error) {
	b := make([]byte, 4)
	_, err := f.Read(b)
	if err != nil {
		return 0, err
	}
	return DeserializeInt32(b), nil
}

func SerializeInt64(number int64) []byte {
	buffer := make([]byte, 8)
	binary.BigEndian.PutUint64(buffer, uint64(number))
	return buffer
}

func DeserializeInt64(bytes []byte) int64 {
	return int64(binary.BigEndian.Uint64(bytes))
}

func Int64FromFile(f *os.File) (int64, error) {
	b := make([]byte, 8)
	_, err := f.Read(b)
	if err != nil {
		return 0, err
	}
	return DeserializeInt64(b), nil
}

func SerializeBool(b bool) []byte {
	if b {
		return []byte{1}
	} else {
		return []byte{0}
	}
}

func DeserializeBool(bytes []byte) bool {
	return bytes[0] == 1
}

func BoolFromFile(f *os.File) (bool, error) {
	b := make([]byte, 1)
	_, err := f.Read(b)
	if err != nil {
		return false, err
	}
	return DeserializeBool(b), nil
}

func SerializedStringLength(s string, maxLength int) int {
	if len(s) == 0 {
		return SizeStringLength
	} else {
		length := len([]byte(s))
		if length > maxLength {
			length = maxLength
		}
		return SizeStringLength + length
	}
}

func SerializeString(s string, maxLength int) []byte {
	var serialized []byte
	if len(s) == 0 {
		serialized = append(serialized, SerializeInt16(0)...)
	} else {
		stringBytes := []byte(s)
		length := len(stringBytes)
		if length > maxLength {
			length = maxLength
		}
		serialized = append(serialized, SerializeInt16(int16(length))...)
		serialized = append(serialized, stringBytes[:length]...)
	}
	return serialized
}

func DeserializeString(bytes []byte) (string, int) {
	if len(bytes) < SizeStringLength {
		return "", 0
	}
	length := int(DeserializeInt16(bytes[0:2]))
	if length == 0 {
		return "", 2
	}
	if len(bytes) < length+2 {
		return "", 2
	}
	return string(bytes[2 : length+2]), length + 2
}

func IP4BytesToString(bytes []byte) string {
	if len(bytes) < 4 {
		return "0.0.0.0"
	}
	return net.IPv4(bytes[0], bytes[1], bytes[2], bytes[3]).String()
}

func IP4StringToBytes(ip string) []byte {
	parsed := net.ParseIP(ip)
	if parsed == nil {
		return []byte{0, 0, 0, 0}
	}
	return parsed[len(parsed)-4:]
}

func AllZeroes(b []byte) bool {
	for _, v := range b {
		if v != 0 {
			return false
		}
	}
	return true
}

func NodeIdFromFile(f *os.File) ([]byte, error) {
	b := make([]byte, SizeNodeIdentifier)
	_, err := f.Read(b)
	return b, err
}

func HashFromFile(f *os.File) ([]byte, error) {
	b := make([]byte, SizeHash)
	_, err := f.Read(b)
	return b, err
}

func SignatureFromFile(f *os.File) ([]byte, error) {
	b := make([]byte, SizeSignature)
	_, err := f.Read(b)
	return b, err
}

func ByteFromFile(f *os.File) (byte, error) {
	b := make([]byte, 1)
	_, err := f.Read(b)
	return b[0], err
}

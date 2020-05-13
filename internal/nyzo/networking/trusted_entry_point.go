package networking

import (
	"strconv"
	"strings"
)

type TrustedEntryPoint struct {
	Host string
	Port int32
}

// Create a trusted entry point from a string, e.g. "verifier0.nyzo.co:9444".
// Returns nil if something goes wrong.
func TrustedEntryPointFromString(data string) *TrustedEntryPoint {
	split := strings.Split(data, ":")
	if len(split) != 2 {
		return nil
	}
	host := split[0]
	if len(host) == 0 {
		return nil
	}
	port, err := strconv.Atoi(split[1])
	if err != nil || port == 0 {
		return nil
	}
	return &TrustedEntryPoint{host, int32(port)}
}

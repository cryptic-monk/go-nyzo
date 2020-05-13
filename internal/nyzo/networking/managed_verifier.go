/*
A small set of components related to peer connections.
*/
package networking

import (
	"github.com/cryptic-monk/go-nyzo/internal/nyzo/messages/message_content/message_fields"
	"github.com/cryptic-monk/go-nyzo/pkg/identity"
	"strconv"
	"strings"
)

type ManagedVerifier struct {
	Host                       string
	Port                       int32
	Identity                   *identity.Identity
	SentinelTransactionEnabled bool
}

// Create a managed verifier from a string, e.g. "206.189.23.229:9444:f61ef799d1b1c171-977347a70bc6e89e-de9ce9be655ffc7a-965f810c936c3ad0".
// Returns nil if something goes wrong.
func ManagedVerifierFromString(line string) *ManagedVerifier {
	splitLine := strings.Split(line, "#")
	nick := ""
	if len(splitLine) > 1 {
		nick = strings.TrimSpace(splitLine[1])
	}

	splitData := strings.Split(strings.TrimSpace(splitLine[0]), ":")
	if len(splitData) < 3 {
		return nil
	}
	host := splitData[0]
	if len(host) == 0 {
		return nil
	}
	port, err := strconv.Atoi(splitData[1])
	if err != nil || port == 0 {
		return nil
	}
	seed, err := identity.NyzoHexToBytes([]byte(splitData[2]), message_fields.SizeSeed)
	if err != nil {
		return nil
	}
	transactionEnabled := false
	if len(splitData) > 3 && strings.ToLower(splitData[3]) == "y" {
		transactionEnabled = true
	}
	id, err := identity.FromPrivateKey(seed)
	if id != nil && nick != "" {
		id.Nickname = nick
	}
	return &ManagedVerifier{host, int32(port), id, transactionEnabled}
}

package configuration

import (
	"github.com/cryptic-monk/go-nyzo/internal/nyzo/messages/message_content/message_fields"
	"github.com/cryptic-monk/go-nyzo/pkg/identity"
	"testing"
)

func TestLockedAccounts(t *testing.T) {
	b, _ := identity.NyzoHexToBytes([]byte(GenesisVerifierNyzoHex), message_fields.SizeNodeIdentifier)
	if !IsLockedAccount(b) {
		t.Errorf("Genesis verifier account should be locked.")
	}
	b, _ = identity.NyzoHexToBytes([]byte("09f86bba22795915-decffece640ed9f0-2268208f9b1857ee-ceb936dab1eacb05"), message_fields.SizeNodeIdentifier)
	if !IsLockedAccount(b) {
		t.Errorf("Account 09f86bba22795915-decffece640ed9f0-2268208f9b1857ee-ceb936dab1eacb05 should be locked.")
	}
	b, _ = identity.NyzoHexToBytes([]byte(CycleAccountNyzoHex), message_fields.SizeNodeIdentifier)
	if IsLockedAccount(b) {
		t.Errorf("Cycle account should not be locked.")
	}

}

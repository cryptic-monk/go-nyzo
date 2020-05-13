package identity

import (
	"os"
	"path/filepath"
	"testing"
)

// Test reading an identity from a private key file. Implicitly also tests identity format conversions.
// TODO: Not tested: faulty data and file access problems.
func TestFromPrivateKeyFile(t *testing.T) {
	file, _ := filepath.Abs("../../test/test_data/verifier_private_seed")
	identity, err := FromPrivateKeyFile(file)
	if err != nil {
		t.Error("Error reading from private key file: " + err.Error())
	} else {
		want := "ec25e1a9aa7819bf-7748eeeebcae05a1-7739ab9905865351-fa2b971abb660047"
		if identity.PublicHex != want {
			t.Errorf("Public Key Hex format incorrect, got: %s, want: %s.", identity.PublicHex, want)
		}
		want = "f61ef799d1b1c171-977347a70bc6e89e-de9ce9be655ffc7a-965f810c936c3ad0"
		if identity.PrivateHex != want {
			t.Errorf("Private Key Hex format incorrect, got: %s, want: %s.", identity.PublicHex, want)
		}
		want = "key_8fpv.XEhJt5PCVd7GNM6Y9ZvEeD~qm_-vGqwxgQjs3IghxxMfuRm"
		if identity.NyzoStringPrivate != want {
			t.Errorf("Private Key Nyzo String format incorrect, got: %s, want: %s.", identity.PublicHex, want)
		}
		want = "id__8eNCWrDHv1D_uSALZIQL1r5VerLq1pqjkwFICPHZqx17rKd-_1Gt"
		if identity.NyzoStringPublic != want {
			t.Errorf("Public Key Nyzo String format incorrect, got: %s, want: %s.", identity.PublicHex, want)
		}
	}
	file, _ = filepath.Abs("../../test/test_data/new_private_seed")
	untested, _ := filepath.Abs("../../test/test_data/untested_verifier_info")
	_ = os.Remove(file)
	newIdentity, err := New(file, untested)
	if err != nil {
		t.Error("Error generating new identity: " + err.Error())
	} else {
		newIdentityReloaded, err := FromPrivateKeyFile(file)
		if err != nil {
			t.Error("Could not reload newly generated identity: " + err.Error())
		} else {
			if newIdentity.NyzoStringPrivate != newIdentityReloaded.NyzoStringPrivate {
				t.Error("Newly generated identity and reloaded version of it don't match.")
			} else {
				nickname, _ := filepath.Abs("../../test/test_data/nickname")
				newIdentity.LoadNicknameFromFile(nickname)
				if newIdentity.Nickname != "go-nyzo😱" {
					t.Error("Could not load nickname.")
				}
			}
		}
	}
}

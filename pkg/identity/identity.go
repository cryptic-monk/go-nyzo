/*
A Nyzo identity, represented by a private and public key pair.
Offering some convenience conversions to Nyzo hex key notations and Nyzo strings.
A nickname can be added optionally.
*/
package identity

import (
	"crypto/ed25519"
	"encoding/hex"
	"errors"
	"fmt"
	"github.com/cryptic-monk/go-nyzo/internal/nyzo/utilities"
	"io/ioutil"
	"strings"
)

type Identity struct {
	PrivateKey        ed25519.PrivateKey // the private key (native format)
	PublicKey         ed25519.PublicKey  // the public key (native format)
	PrivateHex        string             // hexadecimal dashed representation of the private key
	PublicHex         string             // hexadecimal dashed representation of the public key
	ShortId           string             // a short identifier based on the public key hex (ab03...de78)
	Nickname          string             // the user defined Nyzo nickname, used for display only
	NyzoStringPrivate string             // typed, error-protected encoding of the private key
	NyzoStringPublic  string             // typed, error-protected encoding of the public key
}

// Creates a new identity and saves info about it to the given files.
// Existing files will be overwritten, so the caller must make sure that no private key info gets
// unintentionally lost along the way.
func New(privateKeyFile string, infoFile string) (*Identity, error) {
	identity := Identity{}

	// this uses crypto/rand, which is cryptographically secure
	public, private, err := ed25519.GenerateKey(nil)
	if err != nil {
		message := fmt.Sprintf("could not generate new Nyz identity: %s", err.Error())
		return nil, errors.New(message)
	}
	identity.PrivateKey = private
	identity.PublicKey = public
	identity.addConvenienceDerivatives()

	// write the pk file
	err = utilities.StringToFile(identity.PrivateHex, privateKeyFile)
	if err != nil {
		message := fmt.Sprintf("error writing new Nyzo identity to file: %s, %s", privateKeyFile, err.Error())
		return nil, errors.New(message)
	}

	// write the info file
	err = utilities.StringToFile(identity.NyzoStringPrivate+"\n"+identity.NyzoStringPublic, infoFile)
	if err != nil {
		message := fmt.Sprintf("error writing new Nyzo identity info to file: %s, %s", infoFile, err.Error())
		return nil, errors.New(message)
	}

	return &identity, nil
}

// Load an identity from a private key file.
func FromPrivateKeyFile(privateKeyFile string) (*Identity, error) {
	identity := Identity{}
	data, err := ioutil.ReadFile(privateKeyFile)
	if err != nil {
		message := fmt.Sprintf("Could not read private key data from %s: %s", privateKeyFile, err.Error())
		return nil, errors.New(message)
	}
	key, err := NyzoHexToBytes(data, 32)
	if err != nil {
		message := fmt.Sprintf("Could not convert private key data in %s: %s", privateKeyFile, err.Error())
		return nil, errors.New(message)
	}
	identity.PrivateKey = ed25519.NewKeyFromSeed(key)
	identity.PublicKey = make([]byte, ed25519.PublicKeySize)
	copy(identity.PublicKey, identity.PrivateKey[32:])
	identity.addConvenienceDerivatives()
	return &identity, nil
}

// Generate an ID from a private key directly.
func FromPrivateKey(privateKey []byte) (*Identity, error) {
	identity := Identity{}
	identity.PrivateKey = ed25519.NewKeyFromSeed(privateKey)
	identity.PublicKey = make([]byte, ed25519.PublicKeySize)
	copy(identity.PublicKey, identity.PrivateKey[32:])
	identity.addConvenienceDerivatives()
	return &identity, nil
}

// Loads the user-defined nickname from the given file (if not possible, the ShortId is taken as a nick).
func (id *Identity) LoadNicknameFromFile(file string) {
	data, err := ioutil.ReadFile(file)
	nickname := ""
	if err == nil {
		nickname = strings.TrimSpace(string(data))
	}
	if len(nickname) > 0 {
		id.Nickname = nickname
	} else {
		id.Nickname = id.ShortId
	}
}

// Sign the given data with this identity, returns the signature.
func (id *Identity) Sign(data []byte) []byte {
	return ed25519.Sign(id.PrivateKey, data)
}

// Helper to add convenience derivatives like the Nyzo hex formats or the Nyzo strings to the given identity.
func (id *Identity) addConvenienceDerivatives() {
	id.PrivateHex = BytesToNyzoHex(id.PrivateKey.Seed())
	id.PublicHex = BytesToNyzoHex(id.PublicKey)
	id.NyzoStringPrivate = ToNyzoString(NyzoStringTypePrivateKey, id.PrivateKey.Seed())
	id.NyzoStringPublic = ToNyzoString(NyzoStringTypePublicKey, id.PublicKey)
	id.ShortId = hex.EncodeToString(id.PublicKey[:2]) + "..." + hex.EncodeToString(id.PublicKey[len(id.PublicKey)-2:])
	id.Nickname = id.ShortId
}

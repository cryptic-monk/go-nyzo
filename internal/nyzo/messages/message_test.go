package messages

import (
	"bytes"
	"github.com/cryptic-monk/go-nyzo/pkg/identity"
	"io/ioutil"
	"path/filepath"
	"strconv"
	"testing"
)

// Test message signing (and, implicitly, part of message serialization).
func TestSigning(t *testing.T) {
	id, _ := identity.FromPrivateKeyFile("../../../test/test_data/verifier_messenger_private_seed")
	message := NewLocal(TypeMeshRequest, nil, id)
	message.Timestamp = 1573223268986
	message.Signature = id.Sign(message.SerializeForSigning())
	want := []byte{155, 153, 0, 174, 130, 83, 85, 252, 133, 255, 104, 80, 220, 68, 135, 132, 110, 246, 238, 223, 221, 248, 165, 245, 40, 99, 96, 68, 104, 111, 180, 205, 77, 99, 230, 243, 192, 180, 130, 26, 30, 216, 200, 234, 195, 201, 170, 194, 42, 81, 147, 81, 120, 201, 71, 100, 34, 75, 154, 31, 139, 30, 212, 15}
	if !bytes.Equal(want, message.Signature) {
		t.Errorf("Invalid message signature")
	}
}

// Read dumped messages, check their signatures and occasionally content just to make sure that we always
// get serialization/deserialization right.
func TestIndividualMessages(t *testing.T) {
	toTest := []int16{10, 12, 16, 36, 38, 44, 54, 425} // types to test
	for _, messageType := range toTest {
		fileName, _ := filepath.Abs("../../../test/test_data/" + strconv.Itoa(int(messageType)) + ".raw")
		b, err := ioutil.ReadFile(fileName)
		if err != nil {
			t.Errorf("Could not read message dump for type %d.", messageType)
		}
		_, err = ReadNew(bytes.NewReader(b), "8.8.8.8:9444")
		if err != nil {
			t.Errorf("Could not deserialize message type %d: %s.", messageType, err.Error())
		}
	}
}

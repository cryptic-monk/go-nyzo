/*
Can be used to dump itw ('in the wild') message data to a directory for later analysis/testing.
Also: wannabe mumble rapper.
*/
package test

import (
	"github.com/cryptic-monk/go-nyzo/internal/nyzo/messages/message_content/message_fields"
	"io/ioutil"
	"os"
	"path/filepath"
	"strconv"
)

var (
	dumpingGroundz = "test/test_data"
)

func Dump_eet(bytes []byte) {
	if len(bytes) < 14 {
		return
	}
	messageType := message_fields.DeserializeInt16(bytes[12:14])
	fileName, _ := filepath.Abs(dumpingGroundz + "/" + strconv.Itoa(int(messageType)) + ".raw")
	_, err := os.Stat(fileName)
	if os.IsNotExist(err) {
		_ = ioutil.WriteFile(fileName, bytes, 0644)
	}
}

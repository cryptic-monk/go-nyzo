package key_value_store

import (
	"github.com/cryptic-monk/go-nyzo/internal/nyzo/configuration"
	"github.com/cryptic-monk/go-nyzo/internal/nyzo/interfaces"
	"math/rand"
	"strconv"
	"testing"
	"time"
)

func TestLoadData(t *testing.T) {
	data := ctxt.PersistentData.Retrieve(configuration.WinningIdentifierKey, "blah")
	if data != "6043d29f90374263-1ff666eb37b3b23f-93d344d3a7359cc5-894a3b0cba1b8081" {
		t.Error("Could not retrieve winning identifier from persistent data.")
	}
	data = ctxt.PersistentData.Retrieve(configuration.HaveNodeHistoryKey, "0")
	if data != "1" {
		t.Error("Could not retrieve node history from persistent data.")
	}
	data = ctxt.PersistentData.Retrieve("nonexistent_key", "blah")
	if data != "blah" {
		t.Error("Default value does not work.")
	}
}

func TestSaveData(t *testing.T) {
	rand.Seed(time.Now().UnixNano())
	randomString := strconv.Itoa(rand.Intn(213123))
	ctxt.PersistentData.Store("test_key", randomString)
	data := ctxt.PersistentData.Retrieve("test_key", "blah")
	if data != randomString {
		t.Error("Storing of data did not work.")
	}
	// wait for the storage goroutine to come back
	ctxt.WaitGroup.Wait()
	// reinitialize
	data = ctxt.PersistentData.Retrieve("test_key", "blah")
	if data != randomString {
		t.Error("Storing of data and reinitialization did not work.")
	}
}

var ctxt interfaces.Context

func init() {
	ctxt = interfaces.Context{}
	configuration.DataDirectory = "../../../test/test_data"
	ctxt.PersistentData = NewKeyValueStore(configuration.DataDirectory+"/"+configuration.PersistentDataFileName, ctxt.WaitGroup)
}

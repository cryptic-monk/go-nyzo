package node_manager

import (
	"bufio"
	"fmt"
	"github.com/cryptic-monk/go-nyzo/internal/nyzo/configuration"
	"github.com/cryptic-monk/go-nyzo/internal/nyzo/interfaces"
	"os"
	"strings"
	"testing"
	"time"
)

// do those two persisted node files contain the exact same info?
func persistFilesAreSame(f1, f2 string) bool {
	stringMap := make(map[string]bool)

	count1 := 0
	file1, _ := os.Open(f1)
	defer file1.Close()
	scanner1 := bufio.NewScanner(file1)
	for scanner1.Scan() {
		text := scanner1.Text()
		if len(strings.TrimSpace(text)) > 0 {
			stringMap[text] = true
			count1++
		}
	}

	count2 := 0
	file2, _ := os.Open(f2)
	defer file2.Close()
	scanner2 := bufio.NewScanner(file2)
	for scanner2.Scan() {
		text := scanner2.Text()
		if len(strings.TrimSpace(text)) > 0 {
			_, ok := stringMap[text]
			if !ok {
				fmt.Println(text)
				return false
			} else {
				count2++
			}
		}
	}

	return count1 == count2
}

// Test node persisting/loading
func TestNodePersist(t *testing.T) {
	ctxt := &interfaces.Context{}
	ctxt.NodeManager = NewNodeManager(ctxt)
	ctxt.NodeManager.(*state).loadPersistedNodes("../../../test/test_data/nodes")
	ctxt.NodeManager.(*state).persistNodes("../../../test/test_data/nodes_persisted")
	// we have to wait for the goroutine to write the file
	time.Sleep(1 * time.Second)
	if !persistFilesAreSame("../../../test/test_data/nodes", "../../../test/test_data/nodes_persisted") {
		t.Error("persisted node file does not match original")
	}
}

func TestLoadManagedVerifiers(t *testing.T) {
	configuration.DataDirectory = "../../../test/test_data"
	ctxt := &interfaces.Context{}
	ctxt.NodeManager = NewNodeManager(ctxt)
	err := ctxt.NodeManager.(*state).loadManagedVerifiers()
	if err != nil {
		t.Error("cannot load managed verifiers")
	}
}

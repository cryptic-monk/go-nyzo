/*
Key-value store to persist data to a file.

This is used to store few, but important pieces of information in a way that's compatible with the Java version.

Porting from Java status:
- complete and verified to be compatible
- exception: there's no reset() function, as this part is deprecated in the Java version
- exception: error messages are different
- exception: the actual data conversion happens on the caller side, this is just a string-string store (but compatible with Java), we might add conversion helpers here if needed
*/
package key_value_store

import (
	"bufio"
	"fmt"
	"github.com/cryptic-monk/go-nyzo/internal/nyzo/interfaces"
	"io/ioutil"
	"os"
	"strings"
	"sync"
)

const (
	keyValueStoreFilePermissions = 0644
)

type state struct {
	fileName  string
	waitGroup sync.WaitGroup
	data      map[string]string
	fileLock  sync.Mutex
	dataLock  sync.Mutex
}

// Store
func (s *state) Store(key, value string) {
	// we lock the data and make a copy of it for saving to the file
	s.dataLock.Lock()
	s.data[key] = value
	dataCopy := make(map[string]string)
	for key, value := range s.data {
		dataCopy[key] = value
	}
	s.dataLock.Unlock()
	// store asynchronously for added speed, data is now safe, storage protected by the WaitGroup
	s.waitGroup.Add(1)
	go s.writeKeyValueStore(s.fileName, dataCopy)
}

// Retrieve
func (s *state) Retrieve(key, defaultValue string) string {
	s.dataLock.Lock()
	value, ok := s.data[key]
	s.dataLock.Unlock()
	if ok {
		return value
	} else {
		return defaultValue
	}
}

// Write the node persist file.
func (s *state) writeKeyValueStore(fileName string, data map[string]string) {
	// in the unlikely case where we have a lot of writes after each other, this routine will just
	// wait a bit in the background, null problemo
	defer s.waitGroup.Done()
	s.fileLock.Lock()
	defer s.fileLock.Unlock()
	file, err := os.OpenFile(fileName, os.O_CREATE|os.O_WRONLY, keyValueStoreFilePermissions)
	if err == nil {
		defer file.Close()
		writer := bufio.NewWriter(file)
		for key, value := range data {
			_, _ = writer.WriteString(fmt.Sprintf("%s=%s\n", key, value))
		}
		_ = writer.Flush()
	}
}

// This happens pre- main loop, so no concurrency issues.
func (s *state) loadKeyValueStore(fileName string) {
	data, err := ioutil.ReadFile(fileName)
	if err == nil {
		lines := strings.Split(string(data), "\n")
		for _, line := range lines {
			line = strings.Split(line, "#")[0]
			details := strings.Split(line, "=")
			if len(details) == 2 {
				s.data[strings.ToLower(strings.TrimSpace(details[0]))] = strings.TrimSpace(details[1])
			}
		}
	}
}

// Create a key value store.
func NewKeyValueStore(fileName string, waitGroup sync.WaitGroup) interfaces.KeyValueStoreInterface {
	s := &state{}
	s.fileName = fileName
	s.waitGroup = waitGroup
	s.data = make(map[string]string)
	s.loadKeyValueStore(s.fileName)
	return s
}

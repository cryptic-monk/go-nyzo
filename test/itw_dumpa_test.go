/*
Test the itw message dumper.
*/
package test

import (
	"bytes"
	"io/ioutil"
	"os"
	"path/filepath"
	"strconv"
	"testing"
)

func TestDumpa(t *testing.T) {
	dumpingGroundz = "test_data"
	fileName, _ := filepath.Abs("test_data/" + strconv.Itoa(0) + ".raw")
	_ = os.Remove(fileName)
	Dump_eet([]byte{0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 1})
	Dump_eet([]byte{0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 2})
	b, err := ioutil.ReadFile(fileName)
	if err != nil {
		t.Errorf("Could not read back dumpa file.")
	}
	if !bytes.Equal(b, []byte{0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 1}) {
		t.Errorf("Dumpa file has invalid content.")
	}
}

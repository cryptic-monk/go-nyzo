package blockchain_data

import (
	"testing"
)

func TestShlong(t *testing.T) {
	combined := ToShlong(1, 534234)
	ver, height := FromShlong(combined)
	if ver != 1 || height != 534234 {
		t.Error("Shlong conversion did not work.")
	}
}

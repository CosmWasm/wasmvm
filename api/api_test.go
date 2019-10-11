package api

import "testing"

func TestAdd(t *testing.T) {
	res := add(5, 7)
	if res != 12 {
		t.Fatalf("Unexpected result: %d", res)
	}
}
package api

import "testing"

func TestAdd(t *testing.T) {
	res := Add(5, 7)
	if res != 12 {
		t.Fatalf("Unexpected result: %d", res)
	}
}

func TestGreet(t *testing.T) {
	res := string(Greet([]byte("Fred")))
	if res != "Hello, Fred" {
		t.Fatalf("Bad greet: %s", res)
	}

	res = string(Greet(nil))
	if res != "Hi, <nil>" {
		t.Fatalf("Bad greet: %s", res)
	}
}
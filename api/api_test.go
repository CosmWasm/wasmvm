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
	if res != "Hello, <nil>" {
		t.Fatalf("Bad greet: %s", res)
	}
}

func TestDivide(t *testing.T) {
	res, err := Divide(15, 3)
	if err != nil {
		t.Fatalf("Expected no error, got %s", err)
	}
	if res != 5 {
		t.Fatalf("Unexpected result: %d", res)
	}

	res, err = Divide(6, 0)
	if err == nil {
		t.Fatalf("Expected error, but got none")
	}
	t.Log(err)
	if res != 0 {
		t.Fatalf("Unexpected result: %d", res)
	}
}
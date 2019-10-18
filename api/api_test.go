package api

import (
	"testing"
)

type Lookup struct {
	data map[string]string
}

func NewLookup() *Lookup {
	return &Lookup{data: make(map[string]string)}
}

func (l *Lookup) Get(key []byte) []byte {
	val := l.data[string(key)]
	return []byte(val)
}

func (l *Lookup) Set(key, value []byte) {
	l.data[string(key)] = string(value)
}

func TestDemoDBAccess(t *testing.T) {
	l := NewLookup()
	foo := []byte("foo")
	bar := []byte("bar")
	missing := []byte("missing")
	l.Set(foo, []byte("long text that fills the buffer"))
	l.Set(bar, []byte("short"))

	// long
	if err := UpdateDB(l, foo); err != nil {
		t.Fatalf("unexpected error")
	}
	if string(l.Get(foo)) != "long text that fills the buffer." {
		t.Errorf("Unexpected result (long): %s", string(l.Get(foo)))
	}

	// short
	if err := UpdateDB(l, bar); err != nil {
		t.Fatalf("unexpected error")
	}
	if err := UpdateDB(l, bar); err != nil {
		t.Fatalf("unexpected error")
	}
	if err := UpdateDB(l, bar); err != nil {
		t.Fatalf("unexpected error")
	}
	if string(l.Get(bar)) != "short..." {
		t.Errorf("Unexpected result (short): %s", string(l.Get(bar)))
	}

	// missing
	if err := UpdateDB(l, missing); err != nil {
		t.Fatalf("unexpected error")
	}
	if string(l.Get(missing)) != "." {
		t.Errorf("Unexpected result (missing): %s", string(l.Get(missing)))
	}

	err := UpdateDB(l, nil)
	if err == nil {
		t.Fatalf("expected error")
	}
}

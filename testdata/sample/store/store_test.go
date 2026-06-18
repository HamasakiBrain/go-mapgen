package store

import "testing"

func TestSaveGet(t *testing.T) {
	m := NewMemStore()
	if err := m.Save("k", "v"); err != nil {
		t.Fatal(err)
	}
	got, err := m.Get("k")
	if err != nil || got != "v" {
		t.Fatalf("got %q err %v", got, err)
	}
}

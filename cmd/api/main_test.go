package main

import "testing"

func TestTokenHintMasksLongToken(t *testing.T) {
	got := tokenHint("abcdefghijklmnopqrstuvwxyz")
	want := "abcd...wxyz"
	if got != want {
		t.Fatalf("tokenHint mismatch: got %s want %s", got, want)
	}
}

func TestExtractStringTrimsFirstAvailableKey(t *testing.T) {
	value := extractString(map[string]any{"type": "  lead.created  "}, "missing", "type")
	if value == nil || *value != "lead.created" {
		t.Fatalf("unexpected extracted value")
	}
}

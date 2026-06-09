package security

import "testing"

func TestTokenHintMasksLongToken(t *testing.T) {
	got := TokenHint("abcdefghijklmnopqrstuvwxyz")
	want := "abcd...wxyz"
	if got != want {
		t.Fatalf("TokenHint mismatch: got %s want %s", got, want)
	}
}

func TestExtractStringTrimsFirstAvailableKey(t *testing.T) {
	value := ExtractString(map[string]any{"type": "  lead.created  "}, "missing", "type")
	if value == nil || *value != "lead.created" {
		t.Fatalf("unexpected extracted value")
	}
}

func TestHashStringIsStable(t *testing.T) {
	got := HashString("signalbox")
	want := "45f2aebd240cb351b03dc860bf0f011e556afc25714fd5b234fc9b93090654fb"
	if got != want {
		t.Fatalf("HashString mismatch: got %s want %s", got, want)
	}
}

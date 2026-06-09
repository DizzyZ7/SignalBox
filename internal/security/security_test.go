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
	want := "8df7c8a02598ca6c6776be6008c17a851d7140c5c93af7405492ffb1750fd4e0"
	if got != want {
		t.Fatalf("HashString mismatch: got %s want %s", got, want)
	}
}

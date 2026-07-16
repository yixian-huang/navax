package identity

import (
	"strings"
	"testing"
)

func TestNew(t *testing.T) {
	first, err := New("usr")
	if err != nil {
		t.Fatal(err)
	}
	second, err := New("usr")
	if err != nil {
		t.Fatal(err)
	}
	if first == second || !strings.HasPrefix(first, "usr_") || len(first) != 36 {
		t.Fatalf("unexpected IDs %q and %q", first, second)
	}
}

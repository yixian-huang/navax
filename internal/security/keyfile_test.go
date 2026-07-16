package security

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"
)

func TestLoadOrCreateKeyIsPersistent(t *testing.T) {
	path := filepath.Join(t.TempDir(), "privacy.key")
	first, err := LoadOrCreateKey(path, 32)
	if err != nil {
		t.Fatal(err)
	}
	second, err := LoadOrCreateKey(path, 32)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(first, second) {
		t.Fatal("key changed between reads")
	}
	info, err := os.Stat(path)
	if err != nil || info.Mode().Perm() != 0o600 {
		t.Fatalf("key permissions = %v, %v", info.Mode().Perm(), err)
	}
}

package security

import (
	"crypto/rand"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
)

// LoadOrCreateKey loads a fixed-size local instance key or creates it with
// owner-only permissions. It is intended for non-exported privacy salts.
func LoadOrCreateKey(path string, size int) ([]byte, error) {
	if size < 16 {
		return nil, errors.New("key size must be at least 16 bytes")
	}
	key, err := os.ReadFile(path)
	if err == nil {
		if len(key) != size {
			return nil, fmt.Errorf("key file %s has invalid length", path)
		}
		return key, nil
	}
	if !errors.Is(err, os.ErrNotExist) {
		return nil, err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return nil, err
	}
	file, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
	if errors.Is(err, os.ErrExist) {
		return LoadOrCreateKey(path, size)
	}
	if err != nil {
		return nil, err
	}
	key = make([]byte, size)
	if _, err := io.ReadFull(rand.Reader, key); err != nil {
		_ = file.Close()
		_ = os.Remove(path)
		return nil, err
	}
	if _, err := file.Write(key); err != nil {
		_ = file.Close()
		_ = os.Remove(path)
		return nil, err
	}
	if err := file.Sync(); err != nil {
		_ = file.Close()
		_ = os.Remove(path)
		return nil, err
	}
	if err := file.Close(); err != nil {
		return nil, err
	}
	return key, nil
}

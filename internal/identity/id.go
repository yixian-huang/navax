package identity

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
)

// New returns an opaque 128-bit identifier with a short type prefix.
func New(prefix string) (string, error) {
	if prefix == "" || len(prefix) > 12 {
		return "", errors.New("invalid ID prefix")
	}
	buffer := make([]byte, 16)
	if _, err := rand.Read(buffer); err != nil {
		return "", err
	}
	return prefix + "_" + hex.EncodeToString(buffer), nil
}

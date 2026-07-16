package security

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/base64"
	"errors"
	"fmt"
	"io"
	"strings"
)

const secretVersion = "v1"

type SecretBox struct {
	aead cipher.AEAD
}

func NewSecretBox(key []byte) (*SecretBox, error) {
	if len(key) != 32 {
		return nil, errors.New("secret box requires a 32-byte key")
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, fmt.Errorf("create AES cipher: %w", err)
	}
	aead, err := cipher.NewGCM(block)
	if err != nil {
		return nil, fmt.Errorf("create AES-GCM: %w", err)
	}
	return &SecretBox{aead: aead}, nil
}

func (b *SecretBox) Encrypt(plaintext []byte, context string) (string, error) {
	nonce := make([]byte, b.aead.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return "", fmt.Errorf("generate encryption nonce: %w", err)
	}
	sealed := b.aead.Seal(nil, nonce, plaintext, []byte(context))
	payload := append(nonce, sealed...)
	return secretVersion + "." + base64.RawURLEncoding.EncodeToString(payload), nil
}

func (b *SecretBox) Decrypt(encoded, context string) ([]byte, error) {
	version, payload, ok := strings.Cut(encoded, ".")
	if !ok || version != secretVersion {
		return nil, errors.New("unsupported encrypted secret format")
	}
	decoded, err := base64.RawURLEncoding.DecodeString(payload)
	if err != nil || len(decoded) <= b.aead.NonceSize() {
		return nil, errors.New("invalid encrypted secret")
	}
	nonce, ciphertext := decoded[:b.aead.NonceSize()], decoded[b.aead.NonceSize():]
	plaintext, err := b.aead.Open(nil, nonce, ciphertext, []byte(context))
	if err != nil {
		return nil, errors.New("decrypt secret: authentication failed")
	}
	return plaintext, nil
}

package security

import (
	"bytes"
	"testing"
)

func TestPasswordRoundTrip(t *testing.T) {
	encoded, err := HashPassword("correct horse battery staple")
	if err != nil {
		t.Fatal(err)
	}
	valid, err := VerifyPassword(encoded, "correct horse battery staple")
	if err != nil || !valid {
		t.Fatalf("VerifyPassword() = %v, %v", valid, err)
	}
	valid, err = VerifyPassword(encoded, "wrong")
	if err != nil || valid {
		t.Fatalf("wrong password VerifyPassword() = %v, %v", valid, err)
	}
}

func TestTokenIsOpaqueAndHashStable(t *testing.T) {
	plain, hash, err := NewToken()
	if err != nil {
		t.Fatal(err)
	}
	if plain == hash || HashToken(plain) != hash || len(hash) != 64 {
		t.Fatal("token hashing invariant failed")
	}
}

func TestSecretBoxAuthenticatesContext(t *testing.T) {
	key := bytes.Repeat([]byte{7}, 32)
	box, err := NewSecretBox(key)
	if err != nil {
		t.Fatal(err)
	}
	encoded, err := box.Encrypt([]byte("smtp-password"), "provider:smtp")
	if err != nil {
		t.Fatal(err)
	}
	plaintext, err := box.Decrypt(encoded, "provider:smtp")
	if err != nil || string(plaintext) != "smtp-password" {
		t.Fatalf("Decrypt() = %q, %v", plaintext, err)
	}
	if _, err := box.Decrypt(encoded, "provider:dns"); err == nil {
		t.Fatal("Decrypt() accepted the wrong context")
	}
}

package credential

import (
	"crypto/rand"
	"encoding/base64"
	"testing"
)

func testCipher(t *testing.T) *Cipher {
	t.Helper()
	key := make([]byte, 32)
	if _, err := rand.Read(key); err != nil {
		t.Fatal(err)
	}
	cipher, err := New(base64.StdEncoding.EncodeToString(key))
	if err != nil {
		t.Fatal(err)
	}
	return cipher
}

func TestRoundTrip(t *testing.T) {
	cipher := testCipher(t)
	encrypted, err := cipher.Encrypt("line-secret")
	if err != nil {
		t.Fatal(err)
	}
	if encrypted == "line-secret" {
		t.Fatal("credential was stored as plaintext")
	}
	decrypted, err := cipher.Decrypt(encrypted)
	if err != nil {
		t.Fatal(err)
	}
	if decrypted != "line-secret" {
		t.Fatalf("Decrypt() = %q", decrypted)
	}
}

func TestRejectsTampering(t *testing.T) {
	cipher := testCipher(t)
	encrypted, _ := cipher.Encrypt("line-secret")
	if _, err := cipher.Decrypt(encrypted + "x"); err == nil {
		t.Fatal("Decrypt() accepted tampered ciphertext")
	}
}

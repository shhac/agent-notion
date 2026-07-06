package auth

import (
	"bytes"
	"crypto/aes"
	"crypto/cipher"
	"testing"
)

// encryptCBC mirrors Chromium's cookie encryption for round-trip tests.
func encryptCBC(t *testing.T, plain []byte, password string, iterations int) []byte {
	t.Helper()
	key := pbkdf2SHA1([]byte(password), []byte("saltysalt"), iterations, 16)
	block, err := aes.NewCipher(key)
	if err != nil {
		t.Fatal(err)
	}
	pad := aes.BlockSize - len(plain)%aes.BlockSize
	padded := append(append([]byte{}, plain...), bytes.Repeat([]byte{byte(pad)}, pad)...)
	out := make([]byte, len(padded))
	cipher.NewCBCEncrypter(block, bytes16Spaces()).CryptBlocks(out, padded)
	return out
}

func TestDecryptChromiumCBCRoundTrip(t *testing.T) {
	const token = "v02%3Auser_token_associated_secret%3Aabcdef123456"
	enc := encryptCBC(t, []byte(token), "test-pass", 1003)

	plain, err := decryptChromiumCBC(enc, "test-pass", 1003)
	if err != nil {
		t.Fatalf("decrypt: %v", err)
	}
	if got := normalizeCookiePlaintext(plain, 0); got != "v02:user_token_associated_secret:abcdef123456" {
		t.Errorf("got %q", got)
	}
}

func TestNormalizeStripsMetaV24Prefix(t *testing.T) {
	prefix := bytes.Repeat([]byte{0xAB}, 32) // 32-byte SHA-256(host) hash
	plain := append(prefix, []byte("token-abc")...)

	if got := normalizeCookiePlaintext(plain, 24); got != "token-abc" {
		t.Errorf("v24 strip: got %q", got)
	}
	// Below v24, no prefix is present, so nothing is stripped.
	if got := normalizeCookiePlaintext([]byte("token-abc"), 20); got != "token-abc" {
		t.Errorf("pre-v24: got %q", got)
	}
}

func TestNormalizeURLDecodes(t *testing.T) {
	if got := normalizeCookiePlaintext([]byte("a%3Ab%2Fc"), 0); got != "a:b/c" {
		t.Errorf("got %q", got)
	}
}

func TestPKCS7UnpadRejectsBadPadding(t *testing.T) {
	if _, err := pkcs7Unpad([]byte{1, 2, 3, 9}, 4); err == nil {
		t.Error("expected error for pad byte > block")
	}
	if _, err := pkcs7Unpad(nil, 16); err == nil {
		t.Error("expected error for empty input")
	}
}

func TestPBKDF2KnownAnswer(t *testing.T) {
	// Chromium's fixed salt/iterations produce a stable 16-byte key.
	k1 := pbkdf2SHA1([]byte("peanuts"), []byte("saltysalt"), 1, 16)
	k2 := pbkdf2SHA1([]byte("peanuts"), []byte("saltysalt"), 1, 16)
	if len(k1) != 16 {
		t.Fatalf("key length = %d", len(k1))
	}
	if !bytes.Equal(k1, k2) {
		t.Error("PBKDF2 not deterministic")
	}
	if bytes.Equal(k1, pbkdf2SHA1([]byte("peanuts"), []byte("saltysalt"), 1003, 16)) {
		t.Error("iteration count should change the key")
	}
}

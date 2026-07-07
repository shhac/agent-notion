package auth

import (
	"crypto/aes"
	"crypto/cipher"
	"encoding/base64"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// gcmEncrypt mirrors Chromium's Windows v10 cookie format:
// "v10" || nonce(12) || ciphertext||tag, AES-256-GCM.
func gcmEncrypt(t *testing.T, key, plain []byte) []byte {
	t.Helper()
	block, err := aes.NewCipher(key)
	if err != nil {
		t.Fatal(err)
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		t.Fatal(err)
	}
	nonce := make([]byte, 12)
	for i := range nonce {
		nonce[i] = byte(i + 1)
	}
	ct := gcm.Seal(nil, nonce, plain, nil)
	return append(append([]byte("v10"), nonce...), ct...)
}

func TestDecryptChromiumGCMRoundTrip(t *testing.T) {
	key := make([]byte, 32) // AES-256
	for i := range key {
		key[i] = byte(i)
	}
	enc := gcmEncrypt(t, key, []byte("v02:secret-token:xyz"))

	plain, err := decryptChromiumGCM(enc, key)
	if err != nil {
		t.Fatalf("decrypt: %v", err)
	}
	if string(plain) != "v02:secret-token:xyz" {
		t.Errorf("got %q", plain)
	}
}

func TestDecryptChromiumGCMShortCookie(t *testing.T) {
	if _, err := decryptChromiumGCM([]byte("v10tooshort"), make([]byte, 32)); err == nil {
		t.Error("expected error for a cookie shorter than prefix+nonce+tag")
	}
}

func TestParseLocalStateKey(t *testing.T) {
	appb := base64.StdEncoding.EncodeToString([]byte("APPB" + strings.Repeat("x", 20)))
	dpapi := base64.StdEncoding.EncodeToString([]byte("DPAPI" + strings.Repeat("x", 20)))
	bare := base64.StdEncoding.EncodeToString([]byte(strings.Repeat("x", 20)))

	tests := []struct {
		name    string
		raw     string
		wantErr string
	}{
		{"bad json", `{not valid json`, "invalid character"},
		{"missing key", `{"os_crypt":{}}`, "no os_crypt.encrypted_key"},
		{"app-bound", `{"os_crypt":{"encrypted_key":"` + appb + `"}}`, "app-bound"},
		{"bad base64", `{"os_crypt":{"encrypted_key":"!!!not-base64!!!"}}`, "illegal base64"},
		{"dpapi off windows", `{"os_crypt":{"encrypted_key":"` + dpapi + `"}}`, "only available on Windows"},
		{"bare blob off windows", `{"os_crypt":{"encrypted_key":"` + bare + `"}}`, "only available on Windows"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := parseLocalStateKey([]byte(tt.raw))
			if err == nil || !strings.Contains(err.Error(), tt.wantErr) {
				t.Errorf("err = %v, want contains %q", err, tt.wantErr)
			}
		})
	}
}

func TestFindLocalState(t *testing.T) {
	t.Run("network subdir walk-up", func(t *testing.T) {
		base := t.TempDir()
		if err := os.WriteFile(filepath.Join(base, "Local State"), []byte("{}"), 0o600); err != nil {
			t.Fatal(err)
		}
		network := filepath.Join(base, "Network")
		if err := os.MkdirAll(network, 0o755); err != nil {
			t.Fatal(err)
		}
		got, err := findLocalState(filepath.Join(network, "Cookies"))
		if err != nil {
			t.Fatal(err)
		}
		if got != filepath.Join(base, "Local State") {
			t.Errorf("found %q", got)
		}
	})

	t.Run("not found", func(t *testing.T) {
		if _, err := findLocalState(filepath.Join(t.TempDir(), "Cookies")); err == nil {
			t.Error("expected an error when Local State is absent")
		}
	})
}

func TestDecryptCookieDPAPIAppBoundKey(t *testing.T) {
	dir := t.TempDir()
	appb := base64.StdEncoding.EncodeToString([]byte("APPB" + strings.Repeat("x", 20)))
	writeLocalState(t, filepath.Join(dir, "Local State"), appb)

	// v10 cookie forces the Local-State key path, which hits the APPB error.
	enc := gcmEncrypt(t, make([]byte, 32), []byte("ignored"))
	_, err := decryptCookieDPAPI(filepath.Join(dir, "Cookies"), enc, 0)
	if err == nil || !strings.Contains(err.Error(), "app-bound") {
		t.Errorf("err = %v, want app-bound key error", err)
	}
}

func TestDecryptCookieDPAPIBareBlob(t *testing.T) {
	// A non-v10 value is treated as a raw DPAPI blob; unavailable off Windows.
	_, err := decryptCookieDPAPI(filepath.Join(t.TempDir(), "Cookies"), []byte("rawblobbytes"), 0)
	if err == nil {
		t.Error("expected an error decrypting a raw DPAPI blob")
	}
}

func writeLocalState(t *testing.T, path, encryptedKeyB64 string) {
	t.Helper()
	content := `{"os_crypt":{"encrypted_key":"` + encryptedKeyB64 + `"}}`
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
}

package auth

import (
	"crypto/aes"
	"crypto/cipher"
	"encoding/base64"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
)

// decryptCookieDPAPI decrypts a Windows Chromium cookie. "v10"-prefixed values
// use AES-256-GCM with a key stored (DPAPI-wrapped) in the browser's Local
// State; older values are raw DPAPI blobs.
func decryptCookieDPAPI(cookiesPath string, encrypted []byte, metaVersion int) (string, error) {
	if len(encrypted) >= 3 && string(encrypted[:3]) == "v10" {
		key, err := windowsCookieKey(cookiesPath)
		if err != nil {
			return "", err
		}
		plain, err := decryptChromiumGCM(encrypted, key)
		if err != nil {
			return "", err
		}
		return normalizeCookiePlaintext(plain, metaVersion), nil
	}

	plain, err := dpapiUnprotect(encrypted)
	if err != nil {
		return "", err
	}
	return normalizeCookiePlaintext(plain, metaVersion), nil
}

// decryptChromiumGCM decrypts a v10 Windows cookie: prefix(3) || nonce(12) ||
// ciphertext || tag(16), AES-256-GCM.
func decryptChromiumGCM(encrypted, key []byte) ([]byte, error) {
	if len(encrypted) < 3+12+16 {
		return nil, errors.New("cookie too short for AES-GCM")
	}
	nonce := encrypted[3:15]
	ciphertext := encrypted[15:]

	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}
	return gcm.Open(nil, nonce, ciphertext, nil)
}

// windowsCookieKey reads and DPAPI-unwraps the AES key from the browser's
// Local State file (found by walking up from the Cookies path).
func windowsCookieKey(cookiesPath string) ([]byte, error) {
	statePath, err := findLocalState(cookiesPath)
	if err != nil {
		return nil, err
	}
	raw, err := os.ReadFile(statePath)
	if err != nil {
		return nil, err
	}
	return parseLocalStateKey(raw)
}

func parseLocalStateKey(raw []byte) ([]byte, error) {
	var ls struct {
		OSCrypt struct {
			EncryptedKey string `json:"encrypted_key"`
		} `json:"os_crypt"`
	}
	if err := json.Unmarshal(raw, &ls); err != nil {
		return nil, err
	}
	if ls.OSCrypt.EncryptedKey == "" {
		return nil, errors.New("no os_crypt.encrypted_key in Local State")
	}
	blob, err := base64.StdEncoding.DecodeString(ls.OSCrypt.EncryptedKey)
	if err != nil {
		return nil, err
	}
	switch {
	case len(blob) >= 5 && string(blob[:5]) == "DPAPI":
		return dpapiUnprotect(blob[5:])
	case len(blob) >= 4 && string(blob[:4]) == "APPB":
		return nil, errors.New("app-bound encrypted key (Chrome 127+) cannot be unwrapped from user context")
	default:
		return dpapiUnprotect(blob)
	}
}

// findLocalState walks up from the Cookies path (handles <base>/Network/Cookies
// and <base>/Cookies) to the browser's "Local State" file.
func findLocalState(cookiesPath string) (string, error) {
	dir := filepath.Dir(cookiesPath)
	for i := 0; i < 3; i++ {
		candidate := filepath.Join(dir, "Local State")
		if _, err := os.Stat(candidate); err == nil {
			return candidate, nil
		}
		dir = filepath.Dir(dir)
	}
	return "", errors.New("could not locate the browser's Local State file")
}

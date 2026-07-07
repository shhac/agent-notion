package auth

import (
	"errors"
	"runtime"
	"strconv"
)

// extractChromiumCookie reads and decrypts the Notion token_v2 cookie from a
// Chromium-family Cookies SQLite database. macQueries are the macOS keychain
// Safe Storage services to try for the decryption password.
func extractChromiumCookie(cookiesPath string, macQueries []safeStorageQuery) (string, error) {
	copyPath, cleanup, err := copySqliteForRead(cookiesPath)
	if err != nil {
		return "", err
	}
	defer cleanup()

	metaVersion := readCookieMetaVersion(copyPath)

	rows, err := queryReadonlySqlite(copyPath,
		"select value, encrypted_value from cookies where name = '"+cookieName+
			"' and "+hostClause("host_key")+" order by length(encrypted_value) desc")
	if err != nil {
		return "", err
	}
	if len(rows) == 0 {
		return "", errors.New("no Notion token_v2 cookie found in this browser")
	}
	row := rows[0]

	// Rare: an unencrypted plaintext value.
	if v := rowString(row, "value"); v != "" {
		return v, nil
	}

	encrypted := rowBytes(row, "encrypted_value")
	if len(encrypted) == 0 {
		return "", errors.New("token_v2 cookie had no value")
	}

	if runtime.GOOS == "windows" {
		// Windows wraps the key with DPAPI; Local State is found relative to the
		// original path, not the temp copy.
		return decryptCookieDPAPI(cookiesPath, encrypted, metaVersion)
	}

	prefix := ""
	if len(encrypted) >= 3 {
		prefix = string(encrypted[:3])
	}
	data := encrypted
	if prefix == "v10" || prefix == "v11" {
		data = encrypted[3:]
	}

	passwords := safeStoragePasswordsFn(macQueries, prefix)
	if len(passwords) == 0 {
		return "", errors.New("could not read a Safe Storage password from the OS keychain")
	}
	for _, pw := range passwords {
		if plain, err := decryptChromiumCBC(data, pw, chromiumIterations()); err == nil {
			if tok := normalizeCookiePlaintext(plain, metaVersion); tok != "" {
				return tok, nil
			}
		}
	}
	return "", errors.New("could not decrypt the token_v2 cookie with any Safe Storage password")
}

// readCookieMetaVersion returns the Cookies DB schema version (meta.version),
// or 0 if unavailable. Version ≥24 means decrypted values carry a 32-byte
// SHA-256(host) prefix.
func readCookieMetaVersion(dbPath string) int {
	rows, err := queryReadonlySqlite(dbPath, "select value from meta where key = 'version'")
	if err != nil || len(rows) == 0 {
		return 0
	}
	switch v := rows[0]["value"].(type) {
	case string:
		n, _ := strconv.Atoi(v)
		return n
	case int64:
		return int(v)
	}
	return 0
}

//go:build !windows

package auth

import "errors"

// dpapiUnprotect is a stub off Windows; DPAPI cookies are only decryptable
// there.
func dpapiUnprotect(_ []byte) ([]byte, error) {
	return nil, errors.New("DPAPI cookie decryption is only available on Windows")
}

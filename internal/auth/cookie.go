package auth

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/hmac"
	"crypto/sha1"
	"errors"
)

// decryptChromiumCBC decrypts a Chromium "v10"/"v11" cookie value (already
// stripped of the 3-byte version prefix) using the macOS/Linux scheme:
// PBKDF2-HMAC-SHA1(password, "saltysalt", iterations, 16) as an AES-128-CBC
// key with a 16-space IV and PKCS#7 padding. It returns the unpadded
// plaintext; the caller strips any leading domain-hash prefix and URL-decodes.
func decryptChromiumCBC(data []byte, password string, iterations int) ([]byte, error) {
	if len(data) == 0 {
		return nil, errors.New("empty cookie data")
	}
	if iterations < 1 {
		return nil, errors.New("iterations must be >= 1")
	}
	if len(data)%aes.BlockSize != 0 {
		return nil, errors.New("cookie data is not a multiple of the AES block size")
	}

	key := pbkdf2SHA1([]byte(password), []byte("saltysalt"), iterations, 16)
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}
	plain := make([]byte, len(data))
	cipher.NewCBCDecrypter(block, bytes16Spaces()).CryptBlocks(plain, data)
	return pkcs7Unpad(plain, aes.BlockSize)
}

// normalizeCookiePlaintext turns decrypted cookie bytes into the cookie value:
// Chromium meta-version ≥24 prepends a 32-byte SHA-256(host) hash, which is
// dropped. The remaining bytes are the verbatim cookie value and are returned
// as-is — Notion's token_v2 embeds a percent-encoded prefix (e.g. "v03%3A…")
// that is part of the value, so URL-decoding it would corrupt the token.
func normalizeCookiePlaintext(plain []byte, metaVersion int) string {
	if metaVersion >= 24 && len(plain) >= 32 {
		plain = plain[32:]
	}
	return string(plain)
}

func bytes16Spaces() []byte {
	iv := make([]byte, 16)
	for i := range iv {
		iv[i] = ' '
	}
	return iv
}

func pkcs7Unpad(data []byte, blockSize int) ([]byte, error) {
	if len(data) == 0 || len(data)%blockSize != 0 {
		return nil, errors.New("invalid padded data")
	}
	pad := int(data[len(data)-1])
	if pad == 0 || pad > blockSize || pad > len(data) {
		return nil, errors.New("invalid PKCS#7 padding")
	}
	for _, b := range data[len(data)-pad:] {
		if int(b) != pad {
			return nil, errors.New("invalid PKCS#7 padding bytes")
		}
	}
	return data[:len(data)-pad], nil
}

// pbkdf2SHA1 is a minimal PBKDF2-HMAC-SHA1, avoiding a golang.org/x/crypto
// dependency for this single use.
func pbkdf2SHA1(password, salt []byte, iter, keyLen int) []byte {
	prf := hmac.New(sha1.New, password)
	hashLen := prf.Size()
	numBlocks := (keyLen + hashLen - 1) / hashLen

	out := make([]byte, 0, numBlocks*hashLen)
	buf := make([]byte, 4)
	for block := 1; block <= numBlocks; block++ {
		prf.Reset()
		prf.Write(salt)
		buf[0] = byte(block >> 24)
		buf[1] = byte(block >> 16)
		buf[2] = byte(block >> 8)
		buf[3] = byte(block)
		prf.Write(buf)
		u := prf.Sum(nil)

		t := make([]byte, len(u))
		copy(t, u)
		for n := 1; n < iter; n++ {
			prf.Reset()
			prf.Write(u)
			u = prf.Sum(nil)
			for i := range t {
				t[i] ^= u[i]
			}
		}
		out = append(out, t...)
	}
	return out[:keyLen]
}

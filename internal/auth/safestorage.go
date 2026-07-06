package auth

import (
	"os/exec"
	"runtime"
	"strings"
)

// safeStorageQuery is a macOS keychain generic-password lookup.
type safeStorageQuery struct {
	service string
	account string
}

// safeStoragePasswords returns candidate Safe Storage passwords for the
// Chromium cookie key, in try order. On macOS it reads the keychain services;
// on Linux it tries secret-tool plus Chromium's OSCrypt fallbacks.
func safeStoragePasswords(macQueries []safeStorageQuery, prefix string) []string {
	switch runtime.GOOS {
	case "darwin":
		return dedupe(macSafeStoragePasswords(macQueries))
	case "linux":
		return dedupe(linuxSafeStoragePasswords(macQueries, prefix))
	default:
		return nil
	}
}

func macSafeStoragePasswords(queries []safeStorageQuery) []string {
	var out []string
	for _, q := range queries {
		args := []string{"find-generic-password", "-w", "-s", q.service}
		if q.account != "" {
			args = append(args, "-a", q.account)
		}
		if v, err := exec.Command("security", args...).Output(); err == nil {
			if s := strings.TrimRight(string(v), "\n"); s != "" {
				out = append(out, s)
			}
		}
	}
	return out
}

func linuxSafeStoragePasswords(queries []safeStorageQuery, prefix string) []string {
	var out []string
	for _, q := range queries {
		if v, err := exec.Command("secret-tool", "lookup", "service", q.service).Output(); err == nil {
			if s := strings.TrimRight(string(v), "\n"); s != "" {
				out = append(out, s)
			}
		}
	}
	// Chromium Linux OSCrypt fallbacks (os_crypt_linux.cc): empty for v11, then
	// the hardcoded default "peanuts".
	if prefix == "v11" {
		out = append(out, "")
	}
	out = append(out, "peanuts")
	return out
}

// chromiumIterations is the PBKDF2 iteration count: 1 on Linux, 1003 elsewhere.
func chromiumIterations() int {
	if runtime.GOOS == "linux" {
		return 1
	}
	return 1003
}

func dedupe(in []string) []string {
	seen := make(map[string]bool, len(in))
	var out []string
	for _, s := range in {
		if seen[s] {
			continue
		}
		seen[s] = true
		out = append(out, s)
	}
	return out
}

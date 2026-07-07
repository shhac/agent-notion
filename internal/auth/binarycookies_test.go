package auth

import (
	"encoding/binary"
	"testing"
)

// buildBinaryCookies constructs a minimal single-page, single-record
// Cookies.binarycookies blob for the given domain/name/value.
func buildBinaryCookies(domain, name, value string) []byte {
	// Record: 40-byte fixed header, then NUL-terminated domain/name/value.
	recHeaderLen := 40
	domainOff := recHeaderLen
	nameOff := domainOff + len(domain) + 1
	valueOff := nameOff + len(name) + 1
	recLen := valueOff + len(value) + 1

	rec := make([]byte, recLen)
	binary.LittleEndian.PutUint32(rec[16:20], uint32(domainOff))
	binary.LittleEndian.PutUint32(rec[20:24], uint32(nameOff))
	binary.LittleEndian.PutUint32(rec[28:32], uint32(valueOff))
	copy(rec[domainOff:], domain)
	copy(rec[nameOff:], name)
	copy(rec[valueOff:], value)

	// Page: header (tag, count, offset table) + record.
	pageHeaderLen := 12 // tag(4) + count(4) + one offset(4)
	page := make([]byte, pageHeaderLen+len(rec))
	binary.LittleEndian.PutUint32(page[0:4], 0x00000100)
	binary.LittleEndian.PutUint32(page[4:8], 1)
	binary.LittleEndian.PutUint32(page[8:12], uint32(pageHeaderLen))
	copy(page[pageHeaderLen:], rec)

	// File: magic + page count + page-size table + page.
	out := make([]byte, 0, 12+len(page))
	out = append(out, []byte("cook")...)
	countBuf := make([]byte, 4)
	binary.BigEndian.PutUint32(countBuf, 1)
	out = append(out, countBuf...)
	sizeBuf := make([]byte, 4)
	binary.BigEndian.PutUint32(sizeBuf, uint32(len(page)))
	out = append(out, sizeBuf...)
	out = append(out, page...)
	return out
}

func TestParseBinaryCookies(t *testing.T) {
	blob := buildBinaryCookies(".notion.so", "token_v2", "v02%3Asecret")
	cookies, err := parseBinaryCookies(blob)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if len(cookies) != 1 {
		t.Fatalf("got %d cookies", len(cookies))
	}
	c := cookies[0]
	if c.Domain != ".notion.so" || c.Name != "token_v2" || c.Value != "v02%3Asecret" {
		t.Errorf("got %+v", c)
	}
}

func TestParseBinaryCookiesRejectsBadMagic(t *testing.T) {
	if _, err := parseBinaryCookies([]byte("nope1234")); err == nil {
		t.Error("expected error for bad magic")
	}
}

func TestParseBinaryCookiesRejectsTruncated(t *testing.T) {
	blob := buildBinaryCookies(".notion.so", "token_v2", "x")
	if _, err := parseBinaryCookies(blob[:10]); err == nil {
		t.Error("expected error for truncated file")
	}
}

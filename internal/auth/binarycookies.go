package auth

import (
	"encoding/binary"
	"errors"
	"net/url"
)

type binaryCookie struct {
	Domain string
	Name   string
	Value  string
}

// parseBinaryCookies parses Safari/WebKit Cookies.binarycookies. It recovers
// domain, name, and value for each cookie; all reads are bounds-checked.
func parseBinaryCookies(data []byte) ([]binaryCookie, error) {
	if len(data) < 8 || string(data[:4]) != "cook" {
		return nil, errors.New("not a binarycookies file")
	}
	pageCount := int(binary.BigEndian.Uint32(data[4:8]))
	if pageCount < 0 || pageCount > 1<<20 {
		return nil, errors.New("implausible page count")
	}

	off := 8
	pageSizes := make([]int, pageCount)
	for i := 0; i < pageCount; i++ {
		if off+4 > len(data) {
			return nil, errors.New("truncated page-size table")
		}
		pageSizes[i] = int(binary.BigEndian.Uint32(data[off : off+4]))
		off += 4
	}

	var out []binaryCookie
	for _, size := range pageSizes {
		if size < 0 || off+size > len(data) {
			return nil, errors.New("truncated page")
		}
		cookies, err := parseBinaryCookiePage(data[off : off+size])
		if err != nil {
			return nil, err
		}
		out = append(out, cookies...)
		off += size
	}
	return out, nil
}

func parseBinaryCookiePage(page []byte) ([]binaryCookie, error) {
	if len(page) < 12 || binary.LittleEndian.Uint32(page[:4]) != 0x00000100 {
		return nil, errors.New("bad page header")
	}
	count := int(binary.LittleEndian.Uint32(page[4:8]))
	if count < 0 || 8+count*4 > len(page) {
		return nil, errors.New("bad cookie count")
	}

	var out []binaryCookie
	for i := 0; i < count; i++ {
		start := int(binary.LittleEndian.Uint32(page[8+i*4 : 12+i*4]))
		c, err := parseBinaryCookieRecord(page, start)
		if err != nil {
			return nil, err
		}
		out = append(out, c)
	}
	return out, nil
}

func parseBinaryCookieRecord(page []byte, start int) (binaryCookie, error) {
	if start < 0 || start+40 > len(page) {
		return binaryCookie{}, errors.New("truncated cookie record")
	}
	rec := page[start:]
	domainOff := int(binary.LittleEndian.Uint32(rec[16:20]))
	nameOff := int(binary.LittleEndian.Uint32(rec[20:24]))
	valueOff := int(binary.LittleEndian.Uint32(rec[28:32]))

	return binaryCookie{
		Domain: cstringAt(rec, domainOff),
		Name:   cstringAt(rec, nameOff),
		Value:  cstringAt(rec, valueOff),
	}, nil
}

func cstringAt(rec []byte, off int) string {
	if off < 0 || off >= len(rec) {
		return ""
	}
	end := off
	for end < len(rec) && rec[end] != 0 {
		end++
	}
	return string(rec[off:end])
}

// decodeMaybe URL-decodes a cookie value, returning it unchanged if it is not
// percent-encoded.
func decodeMaybe(v string) (string, error) {
	if decoded, err := url.PathUnescape(v); err == nil {
		return decoded, nil
	}
	return v, nil
}

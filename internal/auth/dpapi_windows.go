package auth

import (
	"unsafe"

	"golang.org/x/sys/windows"
)

// dpapiUnprotect unwraps a DPAPI blob in the current user context.
func dpapiUnprotect(blob []byte) ([]byte, error) {
	if len(blob) == 0 {
		return nil, nil
	}
	in := windows.DataBlob{Size: uint32(len(blob)), Data: &blob[0]}
	var out windows.DataBlob
	if err := windows.CryptUnprotectData(&in, nil, nil, 0, nil, 0, &out); err != nil {
		return nil, err
	}
	defer windows.LocalFree(windows.Handle(unsafe.Pointer(out.Data)))
	plain := make([]byte, out.Size)
	copy(plain, unsafe.Slice(out.Data, out.Size))
	return plain, nil
}

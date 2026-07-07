package auth

import (
	"database/sql"
	"fmt"
	"os"
	"path/filepath"

	_ "modernc.org/sqlite" // pure-Go SQLite driver (no cgo → static binary)
)

// copySqliteForRead snapshots a possibly-locked SQLite DB (and its -wal/-shm
// sidecars) into a temp dir so it can be read while the app holds a lock.
func copySqliteForRead(dbPath string) (copyPath string, cleanup func(), err error) {
	tmpDir, err := os.MkdirTemp("", "agent-notion-sqlite-")
	if err != nil {
		return "", func() {}, err
	}
	cleanup = func() { _ = os.RemoveAll(tmpDir) }

	copyPath = filepath.Join(tmpDir, filepath.Base(dbPath))
	if err := copyFile(dbPath, copyPath); err != nil {
		cleanup()
		return "", func() {}, err
	}
	for _, suffix := range []string{"-wal", "-shm"} {
		_ = copyFile(dbPath+suffix, copyPath+suffix) // best effort
	}
	return copyPath, cleanup, nil
}

func copyFile(src, dst string) error {
	data, err := os.ReadFile(src)
	if err != nil {
		return err
	}
	return os.WriteFile(dst, data, 0o600)
}

func queryReadonlySqlite(dbPath, query string) ([]map[string]any, error) {
	db, err := sql.Open("sqlite", "file:"+dbPath+"?mode=ro&immutable=1")
	if err != nil {
		return nil, err
	}
	defer func() { _ = db.Close() }()

	rows, err := db.Query(query)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()

	cols, err := rows.Columns()
	if err != nil {
		return nil, err
	}

	var out []map[string]any
	for rows.Next() {
		cells := make([]any, len(cols))
		ptrs := make([]any, len(cols))
		for i := range cells {
			ptrs[i] = &cells[i]
		}
		if err := rows.Scan(ptrs...); err != nil {
			return nil, err
		}
		row := make(map[string]any, len(cols))
		for i, c := range cols {
			row[c] = cells[i]
		}
		out = append(out, row)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("sqlite read: %w", err)
	}
	return out, nil
}

// rowString coerces a modernc cell (string or []byte) to string.
func rowString(row map[string]any, col string) string {
	switch v := row[col].(type) {
	case string:
		return v
	case []byte:
		return string(v)
	default:
		return ""
	}
}

// rowBytes coerces a modernc cell to []byte.
func rowBytes(row map[string]any, col string) []byte {
	switch v := row[col].(type) {
	case []byte:
		return v
	case string:
		return []byte(v)
	default:
		return nil
	}
}

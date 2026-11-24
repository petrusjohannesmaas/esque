package sqlitefile

import (
	"bytes"
	"os"
	"path/filepath"
	"sort"
)

var header = []byte("SQLite format 3\x00")

// Find returns the SQLite files in dir, sorted by name.
func Find(dir string) ([]string, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, err
	}
	var found []string
	for _, entry := range entries {
		if !entry.Type().IsRegular() {
			continue
		}
		path := filepath.Join(dir, entry.Name())
		file, err := os.Open(path)
		if err != nil {
			return nil, err
		}
		head := make([]byte, len(header))
		n, err := file.Read(head)
		file.Close()
		if err != nil && n < len(header) {
			continue
		}
		if bytes.Equal(head, header) {
			found = append(found, entry.Name())
		}
	}
	sort.Strings(found)
	return found, nil
}

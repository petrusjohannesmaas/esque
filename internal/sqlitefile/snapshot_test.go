package sqlitefile_test

import (
	"bytes"
	"database/sql"
	"os"
	"path/filepath"
	"testing"

	_ "modernc.org/sqlite"

	"github.com/petrusjohannesmaas/esque/internal/sqlitefile"
)

func TestSnapshotIncludesWALRows(t *testing.T) {
	dir := t.TempDir()
	src := filepath.Join(dir, "live.sqlite")
	db, err := sql.Open("sqlite", src)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	if _, err := db.Exec("PRAGMA journal_mode=WAL"); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec("PRAGMA wal_autocheckpoint=0"); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec("CREATE TABLE sample (id INTEGER PRIMARY KEY, note TEXT)"); err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 3; i++ {
		if _, err := db.Exec("INSERT INTO sample (note) VALUES (?)", "row"); err != nil {
			t.Fatal(err)
		}
	}
	wal, err := os.Stat(src + "-wal")
	if err != nil || wal.Size() == 0 {
		t.Fatalf("expected rows in the -wal file: %v", err)
	}
	before, err := os.ReadFile(src)
	if err != nil {
		t.Fatal(err)
	}

	dst := filepath.Join(dir, "snapshot.sqlite")
	if err := sqlitefile.Snapshot(src, dst); err != nil {
		t.Fatalf("Snapshot: %v", err)
	}

	copied, err := sql.Open("sqlite", dst)
	if err != nil {
		t.Fatal(err)
	}
	defer copied.Close()
	var rows int
	if err := copied.QueryRow("SELECT count(*) FROM sample").Scan(&rows); err != nil {
		t.Fatalf("query snapshot: %v", err)
	}
	if rows != 3 {
		t.Errorf("snapshot has %d rows, want 3", rows)
	}

	after, err := os.ReadFile(src)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(before, after) {
		t.Error("Snapshot changed the source database file")
	}
}

func TestCheckDetectsCorruption(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "valid.sqlite")
	db, err := sql.Open("sqlite", path)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec("PRAGMA page_size=1024"); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec("CREATE TABLE sample (id INTEGER PRIMARY KEY, note TEXT)"); err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 500; i++ {
		if _, err := db.Exec("INSERT INTO sample (note) VALUES (?)", "a row of text used to fill several pages"); err != nil {
			t.Fatal(err)
		}
	}
	if err := db.Close(); err != nil {
		t.Fatal(err)
	}

	if err := sqlitefile.Check(path); err != nil {
		t.Errorf("Check on a valid database: %v", err)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if len(data) < 3*1024 {
		t.Fatalf("database has %d bytes, want at least three pages", len(data))
	}
	copy(data[1024:2048], bytes.Repeat([]byte{0xff}, 1024))
	corrupt := filepath.Join(dir, "corrupt.sqlite")
	if err := os.WriteFile(corrupt, data, 0o600); err != nil {
		t.Fatal(err)
	}
	if err := sqlitefile.Check(corrupt); err == nil {
		t.Error("Check on a corrupt database returned nil")
	}
}

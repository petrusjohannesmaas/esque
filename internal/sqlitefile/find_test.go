package sqlitefile_test

import (
	"database/sql"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	_ "modernc.org/sqlite"

	"github.com/petrusjohannesmaas/esque/internal/sqlitefile"
)

func createSQLite(t *testing.T, path string) {
	t.Helper()
	db, err := sql.Open("sqlite", path)
	if err != nil {
		t.Fatalf("open %s: %v", path, err)
	}
	if _, err := db.Exec("CREATE TABLE sample (id INTEGER PRIMARY KEY)"); err != nil {
		t.Fatalf("create table in %s: %v", path, err)
	}
	if err := db.Close(); err != nil {
		t.Fatalf("close %s: %v", path, err)
	}
}

func TestFindListsOnlySQLiteFiles(t *testing.T) {
	dir := t.TempDir()
	createSQLite(t, filepath.Join(dir, "a.sqlite"))
	createSQLite(t, filepath.Join(dir, "b.db"))
	if err := os.WriteFile(filepath.Join(dir, "notes.txt"), []byte("just notes"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "fake.sqlite"), []byte("not a database"), 0o600); err != nil {
		t.Fatal(err)
	}

	got, err := sqlitefile.Find(dir)
	if err != nil {
		t.Fatalf("Find: %v", err)
	}
	want := []string{"a.sqlite", "b.db"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("Find returned %v, want %v", got, want)
	}
}

package backup_test

import (
	"context"
	"database/sql"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	_ "modernc.org/sqlite"

	"github.com/petrusjohannesmaas/esque/internal/backup"
	"github.com/petrusjohannesmaas/esque/internal/config"
	"github.com/petrusjohannesmaas/esque/internal/crypt"
	"github.com/petrusjohannesmaas/esque/internal/remote"
	"github.com/petrusjohannesmaas/esque/internal/remote/remotetest"
)

// walProject creates blog's directory with a WAL-mode database whose three
// rows are still only in the -wal file.
func walProject(t *testing.T) config.Project {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "blog.sqlite")
	db, err := sql.Open("sqlite", path)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { db.Close() })
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
	wal, err := os.Stat(path + "-wal")
	if err != nil || wal.Size() == 0 {
		t.Fatalf("expected rows in the -wal file: %v", err)
	}
	return config.Project{
		Name:       "blog",
		Dir:        dir,
		DB:         "blog.sqlite",
		Passphrase: "s3cret",
		S3: &config.S3Setup{
			Endpoint:    "https://s3.example.com",
			Region:      "af-south-1",
			Bucket:      "backups",
			Prefix:      "blog",
			AccessKeyID: "key",
			SecretKey:   "secret",
		},
	}
}

func TestBackupUploadsEncryptedSnapshot(t *testing.T) {
	ctx := context.Background()
	project := walProject(t)
	store := remotetest.NewMemory(project.S3.Prefix)

	obj, err := backup.Backup(ctx, project, "blog-1", store)
	if err != nil {
		t.Fatalf("Backup: %v", err)
	}

	list, err := store.List(ctx)
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(list) != 1 {
		t.Fatalf("store holds %d objects, want 1", len(list))
	}
	if list[0].Name != "blog-1.sqlite.enc" {
		t.Errorf("stored name = %q, want blog-1.sqlite.enc", list[0].Name)
	}
	if obj.Name != "blog-1.sqlite.enc" {
		t.Errorf("returned name = %q, want blog-1.sqlite.enc", obj.Name)
	}
	if obj.Size != list[0].Size {
		t.Errorf("returned size = %d, stored size = %d", obj.Size, list[0].Size)
	}

	rc, err := store.Get(ctx, "blog-1.sqlite.enc")
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	defer rc.Close()
	plainPath := filepath.Join(t.TempDir(), "restored.sqlite")
	plain, err := os.Create(plainPath)
	if err != nil {
		t.Fatal(err)
	}
	if err := crypt.Decrypt(plain, rc, project.Passphrase); err != nil {
		plain.Close()
		t.Fatalf("Decrypt: %v", err)
	}
	if err := plain.Close(); err != nil {
		t.Fatal(err)
	}

	restored, err := sql.Open("sqlite", plainPath)
	if err != nil {
		t.Fatal(err)
	}
	defer restored.Close()
	var rows int
	if err := restored.QueryRow("SELECT count(*) FROM sample").Scan(&rows); err != nil {
		t.Fatalf("query restored database: %v", err)
	}
	if rows != 3 {
		t.Errorf("restored database has %d rows, want 3", rows)
	}
}

func TestBackupRejectsBadName(t *testing.T) {
	ctx := context.Background()
	project := walProject(t)
	store := remotetest.NewMemory(project.S3.Prefix)

	_, err := backup.Backup(ctx, project, "my backup!", store)
	if !errors.Is(err, backup.ErrBadName) {
		t.Fatalf("Backup error = %v, want ErrBadName", err)
	}

	list, err := store.List(ctx)
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(list) != 0 {
		t.Errorf("store holds %d objects after a refused name, want none", len(list))
	}
}

// backupTemps lists temp files left by this pipeline.
func backupTemps(t *testing.T) []string {
	t.Helper()
	entries, err := os.ReadDir(os.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	var names []string
	for _, entry := range entries {
		if strings.HasPrefix(entry.Name(), "esque-backup-") {
			names = append(names, entry.Name())
		}
	}
	return names
}

func TestBackupFailureLeavesNothing(t *testing.T) {
	ctx := context.Background()

	t.Run("missing database", func(t *testing.T) {
		project := walProject(t)
		store := remotetest.NewMemory(project.S3.Prefix)
		path := filepath.Join(project.Dir, project.DB)
		if err := os.Remove(path); err != nil {
			t.Fatal(err)
		}
		before := backupTemps(t)

		_, err := backup.Backup(ctx, project, "blog-1", store)
		if !errors.Is(err, backup.ErrDatabaseMissing) {
			t.Fatalf("Backup error = %v, want ErrDatabaseMissing", err)
		}
		if !strings.Contains(err.Error(), path) {
			t.Errorf("error %q does not name %s", err, path)
		}
		assertEmptyStore(t, store)
		if after := backupTemps(t); len(after) != len(before) {
			t.Errorf("temp files left behind: %v", after)
		}
	})

	t.Run("upload refused", func(t *testing.T) {
		project := walProject(t)
		store := remotetest.NewMemory(project.S3.Prefix)
		store.FailPut = remote.ErrAccessDenied
		before := backupTemps(t)

		_, err := backup.Backup(ctx, project, "blog-1", store)
		if !errors.Is(err, remote.ErrAccessDenied) {
			t.Fatalf("Backup error = %v, want remote.ErrAccessDenied", err)
		}
		assertEmptyStore(t, store)
		if after := backupTemps(t); len(after) != len(before) {
			t.Errorf("temp files left behind: %v", after)
		}
	})
}

func assertEmptyStore(t *testing.T, store *remotetest.Memory) {
	t.Helper()
	list, err := store.List(context.Background())
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(list) != 0 {
		t.Errorf("store holds %d objects after a failure, want none", len(list))
	}
}

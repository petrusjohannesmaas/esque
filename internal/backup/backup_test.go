package backup_test

import (
	"bytes"
	"context"
	"crypto/rand"
	"database/sql"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

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
		tmp := t.TempDir()
		t.Setenv("TMPDIR", tmp)
		store := remotetest.NewMemory(project.S3.Prefix)
		path := filepath.Join(project.Dir, project.DB)
		if err := os.Remove(path); err != nil {
			t.Fatal(err)
		}

		_, err := backup.Backup(ctx, project, "blog-1", store)
		if !errors.Is(err, backup.ErrDatabaseMissing) {
			t.Fatalf("Backup error = %v, want ErrDatabaseMissing", err)
		}
		if !strings.Contains(err.Error(), path) {
			t.Errorf("error %q does not name %s", err, path)
		}
		assertEmptyStore(t, store)
		if left := backupTemps(t); len(left) != 0 {
			t.Errorf("temp files left behind: %v", left)
		}
	})

	t.Run("upload refused", func(t *testing.T) {
		project := walProject(t)
		tmp := t.TempDir()
		t.Setenv("TMPDIR", tmp)
		store := remotetest.NewMemory(project.S3.Prefix)
		store.FailPut = remote.ErrAccessDenied

		_, err := backup.Backup(ctx, project, "blog-1", store)
		if !errors.Is(err, remote.ErrAccessDenied) {
			t.Fatalf("Backup error = %v, want remote.ErrAccessDenied", err)
		}
		assertEmptyStore(t, store)
		if left := backupTemps(t); len(left) != 0 {
			t.Errorf("temp files left behind: %v", left)
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

func TestListNewestFirst(t *testing.T) {
	ctx := context.Background()
	store := remotetest.NewMemory("blog")
	objects := []struct {
		name     string
		modified time.Time
		size     int
	}{
		{"a.sqlite.enc", time.Date(2026, 11, 1, 10, 0, 0, 0, time.Local), 10},
		{"c.sqlite.enc", time.Date(2026, 11, 3, 10, 0, 0, 0, time.Local), 30},
		{"b.sqlite.enc", time.Date(2026, 11, 2, 10, 0, 0, 0, time.Local), 20},
		{"notes.txt", time.Date(2026, 11, 4, 10, 0, 0, 0, time.Local), 5},
	}
	for _, obj := range objects {
		data := bytes.Repeat([]byte("x"), obj.size)
		if err := store.Put(ctx, obj.name, bytes.NewReader(data), int64(len(data))); err != nil {
			t.Fatalf("Put %s: %v", obj.name, err)
		}
		store.SetModified(obj.name, obj.modified)
	}

	list, err := backup.List(ctx, store)
	if err != nil {
		t.Fatalf("List: %v", err)
	}

	want := []remote.Object{
		{Name: "c.sqlite.enc", Size: 30, Modified: objects[1].modified},
		{Name: "b.sqlite.enc", Size: 20, Modified: objects[2].modified},
		{Name: "a.sqlite.enc", Size: 10, Modified: objects[0].modified},
	}
	if len(list) != len(want) {
		t.Fatalf("List returned %d objects, want %d: %+v", len(list), len(want), list)
	}
	for i := range want {
		got := list[i]
		if got.Name != want[i].Name {
			t.Errorf("list[%d].Name = %q, want %q", i, got.Name, want[i].Name)
		}
		if got.Size != want[i].Size {
			t.Errorf("list[%d].Size = %d, want %d", i, got.Size, want[i].Size)
		}
		if !got.Modified.Equal(want[i].Modified) {
			t.Errorf("list[%d].Modified = %v, want %v", i, got.Modified, want[i].Modified)
		}
	}
}

func TestRestoreWritesCheckedCopy(t *testing.T) {
	ctx := context.Background()
	project := walProject(t)
	store := remotetest.NewMemory(project.S3.Prefix)
	if _, err := backup.Backup(ctx, project, "blog-1", store); err != nil {
		t.Fatalf("Backup: %v", err)
	}
	livePath := filepath.Join(project.Dir, project.DB)
	before, err := os.ReadFile(livePath)
	if err != nil {
		t.Fatal(err)
	}

	dest := filepath.Join(t.TempDir(), "restored")
	path, err := backup.Restore(ctx, project, "blog-1", dest, store)
	if err != nil {
		t.Fatalf("Restore: %v", err)
	}

	wantPath := filepath.Join(dest, "blog-1.sqlite")
	if path != wantPath {
		t.Errorf("Restore returned %q, want %q", path, wantPath)
	}
	if _, err := os.Stat(wantPath); err != nil {
		t.Fatalf("restored file is missing: %v", err)
	}
	restored, err := sql.Open("sqlite", wantPath)
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

	after, err := os.ReadFile(livePath)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(before, after) {
		t.Error("live database changed during the restore")
	}
}

// encryptBytes returns plaintext encrypted with passphrase.
func encryptBytes(t *testing.T, plain []byte, passphrase string) []byte {
	t.Helper()
	var buf bytes.Buffer
	if err := crypt.Encrypt(&buf, bytes.NewReader(plain), passphrase); err != nil {
		t.Fatalf("Encrypt: %v", err)
	}
	return buf.Bytes()
}

func TestRestoreRefusesBadBackup(t *testing.T) {
	ctx := context.Background()
	junk := make([]byte, 64)
	if _, err := rand.Read(junk); err != nil {
		t.Fatal(err)
	}
	cases := []struct {
		name       string
		passphrase string
		plain      []byte
	}{
		{"wrong", "another passphrase", []byte("not a database")},
		{"junk", "s3cret", junk},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			project := walProject(t)
			store := remotetest.NewMemory(project.S3.Prefix)
			object := tc.name + ".sqlite.enc"
			data := encryptBytes(t, tc.plain, tc.passphrase)
			if err := store.Put(ctx, object, bytes.NewReader(data), int64(len(data))); err != nil {
				t.Fatalf("Put: %v", err)
			}

			dest := filepath.Join(t.TempDir(), "restored")
			if err := os.MkdirAll(dest, 0o700); err != nil {
				t.Fatal(err)
			}
			path, err := backup.Restore(ctx, project, tc.name, dest, store)
			if !errors.Is(err, backup.ErrBadBackup) {
				t.Fatalf("Restore error = %v, want ErrBadBackup", err)
			}
			if path != "" {
				t.Errorf("Restore returned %q with a bad backup, want no path", path)
			}
			entries, err := os.ReadDir(dest)
			if err != nil {
				t.Fatal(err)
			}
			if len(entries) != 0 {
				names := make([]string, 0, len(entries))
				for _, entry := range entries {
					names = append(names, entry.Name())
				}
				t.Errorf("destination holds %v after a bad backup, want no files", names)
			}
		})
	}
}

func TestRestoreKeepsExistingFile(t *testing.T) {
	ctx := context.Background()
	project := walProject(t)
	store := remotetest.NewMemory(project.S3.Prefix)
	if _, err := backup.Backup(ctx, project, "blog-1", store); err != nil {
		t.Fatalf("Backup: %v", err)
	}

	dest := filepath.Join(t.TempDir(), "restored")
	if err := os.MkdirAll(dest, 0o700); err != nil {
		t.Fatal(err)
	}
	existing := filepath.Join(dest, "blog-1.sqlite")
	if err := os.WriteFile(existing, []byte("keep"), 0o600); err != nil {
		t.Fatal(err)
	}

	_, err := backup.Restore(ctx, project, "blog-1", dest, store)
	if !errors.Is(err, backup.ErrFileExists) {
		t.Fatalf("Restore error = %v, want ErrFileExists", err)
	}
	content, err := os.ReadFile(existing)
	if err != nil {
		t.Fatal(err)
	}
	if string(content) != "keep" {
		t.Errorf("existing file holds %q, want keep", content)
	}
}

// Package backup runs the backup pipeline for one project.
package backup

import (
	"context"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"time"

	"github.com/petrusjohannesmaas/esque/internal/config"
	"github.com/petrusjohannesmaas/esque/internal/crypt"
	"github.com/petrusjohannesmaas/esque/internal/remote"
	"github.com/petrusjohannesmaas/esque/internal/sqlitefile"
)

// Errors the backup pipeline classifies for the screen.
var (
	ErrBadName         = errors.New("backup name may use only letters, digits, -, _ and .")
	ErrNameExists      = errors.New("backup name already exists")
	ErrDatabaseMissing = errors.New("database file is missing")
)

// namePattern is what a backup name may contain.
var namePattern = regexp.MustCompile(`^[A-Za-z0-9._-]+$`)

// ValidName returns ErrBadName when name has a character a backup name may not
// use.
func ValidName(name string) error {
	if name == "" || !namePattern.MatchString(name) {
		return fmt.Errorf("%q: %w", name, ErrBadName)
	}
	return nil
}

// DefaultName returns the backup name suggested for a project at now, such as
// blog-2026-11-25_14-30-00.
func DefaultName(project string, now time.Time) string {
	return fmt.Sprintf("%s-%s", project, now.Format("2006-01-02_15-04-05"))
}

// Backup snapshots, encrypts and uploads one project's database as
// <name>.sqlite.enc.
func Backup(ctx context.Context, project config.Project, name string, store remote.Store) (remote.Object, error) {
	var none remote.Object
	if err := ValidName(name); err != nil {
		return none, err
	}
	object := name + ".sqlite.enc"
	exists, err := store.Exists(ctx, object)
	if err != nil {
		return none, fmt.Errorf("check %s: %w", object, err)
	}
	if exists {
		return none, fmt.Errorf("%s: %w", object, ErrNameExists)
	}
	dbPath := filepath.Join(project.Dir, project.DB)
	if _, err := os.Stat(dbPath); errors.Is(err, fs.ErrNotExist) {
		return none, fmt.Errorf("%w: %s", ErrDatabaseMissing, dbPath)
	} else if err != nil {
		return none, err
	}

	snapshot, err := os.CreateTemp("", "esque-backup-*.sqlite")
	if err != nil {
		return none, err
	}
	snapshotPath := snapshot.Name()
	defer os.Remove(snapshotPath)
	if err := snapshot.Chmod(0o600); err != nil {
		snapshot.Close()
		return none, err
	}
	if err := snapshot.Close(); err != nil {
		return none, err
	}
	if err := sqlitefile.Snapshot(dbPath, snapshotPath); err != nil {
		return none, fmt.Errorf("snapshot %s: %w", dbPath, err)
	}

	encrypted, err := os.CreateTemp("", "esque-backup-*.enc")
	if err != nil {
		return none, err
	}
	encryptedPath := encrypted.Name()
	defer func() {
		encrypted.Close()
		os.Remove(encryptedPath)
	}()
	source, err := os.Open(snapshotPath)
	if err != nil {
		return none, err
	}
	defer source.Close()
	if err := crypt.Encrypt(encrypted, source, project.Passphrase); err != nil {
		return none, fmt.Errorf("encrypt %s: %w", snapshotPath, err)
	}
	info, err := encrypted.Stat()
	if err != nil {
		return none, err
	}
	if _, err := encrypted.Seek(0, io.SeekStart); err != nil {
		return none, err
	}
	if err := store.Put(ctx, object, encrypted, info.Size()); err != nil {
		return none, fmt.Errorf("upload %s: %w", object, err)
	}
	return remote.Object{Name: object, Size: info.Size()}, nil
}

package sqlitefile

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"net/url"
	"strings"

	"modernc.org/sqlite"
)

// backuper is the part of modernc's driver connection that starts a backup.
type backuper interface {
	NewBackup(dst string) (*sqlite.Backup, error)
}

// readOnlyURI builds a read-only SQLite URI, escaping characters such as
// #, ? and % in the path.
func readOnlyURI(path string) string {
	u := url.URL{Scheme: "file", Path: path, RawQuery: "mode=ro"}
	return u.String()
}

// Snapshot copies the database at src to dst, including data still in the
// -wal file, without writing to src.
func Snapshot(src, dst string) error {
	db, err := sql.Open("sqlite", readOnlyURI(src))
	if err != nil {
		return err
	}
	defer db.Close()
	conn, err := db.Conn(context.Background())
	if err != nil {
		return err
	}
	defer conn.Close()

	var backup *sqlite.Backup
	err = conn.Raw(func(driverConn any) error {
		source, ok := driverConn.(backuper)
		if !ok {
			return errors.New("sqlitefile: driver does not support backups")
		}
		backup, err = source.NewBackup(dst)
		return err
	})
	if err != nil {
		return err
	}
	if _, err := backup.Step(-1); err != nil {
		backup.Finish()
		return err
	}
	return backup.Finish()
}

// Check runs SQLite's integrity check on the database at path.
func Check(path string) error {
	db, err := sql.Open("sqlite", readOnlyURI(path))
	if err != nil {
		return err
	}
	defer db.Close()
	rows, err := db.Query("PRAGMA integrity_check")
	if err != nil {
		return err
	}
	defer rows.Close()
	var results []string
	for rows.Next() {
		var line string
		if err := rows.Scan(&line); err != nil {
			return err
		}
		results = append(results, line)
	}
	if err := rows.Err(); err != nil {
		return err
	}
	if len(results) != 1 || results[0] != "ok" {
		return fmt.Errorf("sqlitefile: integrity check of %s: %s", path, strings.Join(results, "; "))
	}
	return nil
}

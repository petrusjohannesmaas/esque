// Package remote stores and reads backup objects in an S3-compatible service.
package remote

import (
	"context"
	"errors"
	"io"
	"time"
)

// Errors a Store returns for the failures the interface cares about.
var (
	ErrAccessDenied = errors.New("access denied")
	ErrUnreachable  = errors.New("service unreachable")
	ErrNotFound     = errors.New("object not found")
)

// Object is one stored item, with a name relative to the store's prefix.
type Object struct {
	Name     string
	Size     int64
	Modified time.Time
}

// Store holds the backups of one project under its prefix.
type Store interface {
	Put(ctx context.Context, name string, r io.Reader, size int64) error
	Exists(ctx context.Context, name string) (bool, error)
	List(ctx context.Context) ([]Object, error)
	Get(ctx context.Context, name string) (io.ReadCloser, error)
}

package remote

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"net/url"
	"strings"

	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"

	"github.com/petrusjohannesmaas/esque/internal/config"
)

// NewS3 returns a Store backed by an S3-compatible service.
func NewS3(setup config.S3Setup) (Store, error) {
	endpoint := setup.Endpoint
	secure := true
	if rest, ok := strings.CutPrefix(endpoint, "http://"); ok {
		endpoint, secure = rest, false
	} else if rest, ok := strings.CutPrefix(endpoint, "https://"); ok {
		endpoint = rest
	}
	endpoint = strings.TrimSuffix(endpoint, "/")

	client, err := minio.New(endpoint, &minio.Options{
		Creds:        credentials.NewStaticV4(setup.AccessKeyID, setup.SecretKey, ""),
		Secure:       secure,
		Region:       setup.Region,
		BucketLookup: minio.BucketLookupPath,
	})
	if err != nil {
		return nil, err
	}
	return &s3Store{client: client, bucket: setup.Bucket, prefix: setup.Prefix}, nil
}

type s3Store struct {
	client *minio.Client
	bucket string
	prefix string
}

func (s *s3Store) key(name string) string {
	if s.prefix == "" {
		return name
	}
	return strings.TrimSuffix(s.prefix, "/") + "/" + name
}

func (s *s3Store) folder() string {
	if s.prefix == "" {
		return ""
	}
	return strings.TrimSuffix(s.prefix, "/") + "/"
}

func (s *s3Store) Put(ctx context.Context, name string, r io.Reader, size int64) error {
	_, err := s.client.PutObject(ctx, s.bucket, s.key(name), r, size, minio.PutObjectOptions{})
	return mapError(err)
}

func (s *s3Store) Exists(ctx context.Context, name string) (bool, error) {
	_, err := s.client.StatObject(ctx, s.bucket, s.key(name), minio.StatObjectOptions{})
	if err == nil {
		return true, nil
	}
	if errors.Is(mapError(err), ErrNotFound) {
		return false, nil
	}
	return false, mapError(err)
}

func (s *s3Store) List(ctx context.Context) ([]Object, error) {
	folder := s.folder()
	var list []Object
	for info := range s.client.ListObjects(ctx, s.bucket, minio.ListObjectsOptions{Prefix: folder, Recursive: false}) {
		if info.Err != nil {
			return nil, mapError(info.Err)
		}
		name := strings.TrimPrefix(info.Key, folder)
		if name == "" {
			continue
		}
		list = append(list, Object{Name: name, Size: info.Size, Modified: info.LastModified})
	}
	return list, nil
}

func (s *s3Store) Get(ctx context.Context, name string) (io.ReadCloser, error) {
	obj, err := s.client.GetObject(ctx, s.bucket, s.key(name), minio.GetObjectOptions{})
	if err != nil {
		return nil, err
	}
	if _, err := obj.Stat(); err != nil {
		obj.Close()
		return nil, mapError(err)
	}
	return obj, nil
}

// mapError turns the minio and network errors the interface cares about into
// its own sentinels, keeping the original message.
func mapError(err error) error {
	if err == nil {
		return nil
	}
	resp := minio.ToErrorResponse(err)
	switch resp.Code {
	case "NoSuchKey", "NoSuchBucket", "NotFound":
		return fmt.Errorf("%w: %s", ErrNotFound, resp.Message)
	case "InvalidAccessKeyId", "SignatureDoesNotMatch", "AccessDenied", "InvalidSecurity":
		return fmt.Errorf("%w: %s", ErrAccessDenied, resp.Message)
	}
	var urlErr *url.Error
	var netErr net.Error
	if errors.As(err, &urlErr) || errors.As(err, &netErr) {
		return fmt.Errorf("%w: %w", ErrUnreachable, err)
	}
	return err
}

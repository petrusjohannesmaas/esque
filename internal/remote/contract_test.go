package remote_test

import (
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"

	"github.com/johannesboyne/gofakes3"
	"github.com/johannesboyne/gofakes3/backend/s3mem"
	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"

	"github.com/petrusjohannesmaas/esque/internal/config"
	"github.com/petrusjohannesmaas/esque/internal/remote"
	"github.com/petrusjohannesmaas/esque/internal/remote/remotetest"
)

func TestMemoryStore(t *testing.T) {
	bucket := remotetest.NewMemory("")
	remotetest.Contract(t, func(prefix string) remote.Store {
		return bucket.WithPrefix(prefix)
	})
}

func TestS3Store(t *testing.T) {
	setup := fakeS3(t)
	remotetest.Contract(t, func(prefix string) remote.Store {
		withPrefix := setup
		withPrefix.Prefix = prefix
		store, err := remote.NewS3(withPrefix)
		if err != nil {
			t.Fatalf("NewS3: %v", err)
		}
		return store
	})
}

func TestS3StoreErrors(t *testing.T) {
	t.Run("AccessDenied", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/xml")
			w.WriteHeader(http.StatusForbidden)
			io.WriteString(w, `<?xml version="1.0" encoding="UTF-8"?><Error><Code>InvalidAccessKeyId</Code><Message>the key is not known</Message></Error>`)
		}))
		t.Cleanup(server.Close)

		store, err := remote.NewS3(config.S3Setup{
			Endpoint:    server.URL,
			Region:      "us-east-1",
			Bucket:      "backups",
			AccessKeyID: "AKID",
			SecretKey:   "secret",
		})
		if err != nil {
			t.Fatalf("NewS3: %v", err)
		}
		if _, err := store.List(context.Background()); !errors.Is(err, remote.ErrAccessDenied) {
			t.Errorf("List error = %v, want ErrAccessDenied", err)
		}
	})

	t.Run("Unreachable", func(t *testing.T) {
		server := httptest.NewServer(http.NotFoundHandler())
		endpoint := server.URL
		server.Close()

		store, err := remote.NewS3(config.S3Setup{
			Endpoint:    endpoint,
			Region:      "us-east-1",
			Bucket:      "backups",
			AccessKeyID: "AKID",
			SecretKey:   "secret",
		})
		if err != nil {
			t.Fatalf("NewS3: %v", err)
		}
		if _, err := store.List(context.Background()); !errors.Is(err, remote.ErrUnreachable) {
			t.Errorf("List error = %v, want ErrUnreachable", err)
		}
	})
}

// fakeS3 starts gofakes3 with an empty backups bucket and returns a matching
// S3 setup without a prefix.
func fakeS3(t *testing.T) config.S3Setup {
	t.Helper()
	server := httptest.NewServer(gofakes3.New(s3mem.New()).Server())
	t.Cleanup(server.Close)
	u, err := url.Parse(server.URL)
	if err != nil {
		t.Fatalf("parse server URL: %v", err)
	}
	client, err := minio.New(u.Host, &minio.Options{
		Creds:        credentials.NewStaticV4("ACCESS_KEY", "SECRET_KEY", ""),
		Secure:       false,
		Region:       "us-east-1",
		BucketLookup: minio.BucketLookupPath,
	})
	if err != nil {
		t.Fatalf("minio.New: %v", err)
	}
	if err := client.MakeBucket(context.Background(), "backups", minio.MakeBucketOptions{Region: "us-east-1"}); err != nil {
		t.Fatalf("MakeBucket: %v", err)
	}
	return config.S3Setup{
		Endpoint:    server.URL,
		Region:      "us-east-1",
		Bucket:      "backups",
		AccessKeyID: "ACCESS_KEY",
		SecretKey:   "SECRET_KEY",
	}
}

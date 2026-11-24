// Package remotetest holds an in-memory remote.Store and the contract suite
// that every adapter runs.
package remotetest

import (
	"bytes"
	"context"
	"io"
	"testing"

	"github.com/petrusjohannesmaas/esque/internal/remote"
)

// Contract runs the store contract against newStore. Every store it returns
// writes to the same bucket, so prefixes must keep them apart.
func Contract(t *testing.T, newStore func(prefix string) remote.Store) {
	t.Helper()

	t.Run("PutListGet", func(t *testing.T) {
		ctx := context.Background()
		store := newStore("srv1/blog")
		if err := store.Put(ctx, "a.sqlite.enc", bytes.NewReader([]byte("abc")), 3); err != nil {
			t.Fatalf("Put: %v", err)
		}

		list, err := store.List(ctx)
		if err != nil {
			t.Fatalf("List: %v", err)
		}
		if len(list) != 1 {
			t.Fatalf("List returned %d objects, want 1", len(list))
		}
		if list[0].Name != "a.sqlite.enc" {
			t.Errorf("listed name = %q, want a.sqlite.enc", list[0].Name)
		}
		if list[0].Size != 3 {
			t.Errorf("listed size = %d, want 3", list[0].Size)
		}
		if list[0].Modified.IsZero() {
			t.Error("listed object has no modified time")
		}

		ok, err := store.Exists(ctx, "a.sqlite.enc")
		if err != nil {
			t.Fatalf("Exists: %v", err)
		}
		if !ok {
			t.Error("Exists = false after Put")
		}

		rc, err := store.Get(ctx, "a.sqlite.enc")
		if err != nil {
			t.Fatalf("Get: %v", err)
		}
		defer rc.Close()
		got, err := io.ReadAll(rc)
		if err != nil {
			t.Fatalf("read object: %v", err)
		}
		if string(got) != "abc" {
			t.Errorf("Get returned %q, want abc", got)
		}
	})

	t.Run("PrefixesAreSeparate", func(t *testing.T) {
		ctx := context.Background()
		blog := newStore("srv1/blog")
		shop := newStore("srv1/shop")
		if err := blog.Put(ctx, "a.sqlite.enc", bytes.NewReader([]byte("blog")), 4); err != nil {
			t.Fatalf("Put in blog: %v", err)
		}
		if err := shop.Put(ctx, "b.sqlite.enc", bytes.NewReader([]byte("shop")), 4); err != nil {
			t.Fatalf("Put in shop: %v", err)
		}

		for _, tc := range []struct {
			prefix string
			store  remote.Store
			want   string
		}{
			{"srv1/blog", blog, "a.sqlite.enc"},
			{"srv1/shop", shop, "b.sqlite.enc"},
		} {
			list, err := tc.store.List(ctx)
			if err != nil {
				t.Fatalf("List under %s: %v", tc.prefix, err)
			}
			if len(list) != 1 {
				t.Errorf("List under %s returned %d objects, want 1", tc.prefix, len(list))
				continue
			}
			if list[0].Name != tc.want {
				t.Errorf("List under %s = %q, want %q", tc.prefix, list[0].Name, tc.want)
			}
		}
	})
}

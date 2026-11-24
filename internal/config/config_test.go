package config_test

import (
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/petrusjohannesmaas/esque/internal/config"
)

func TestSaveWritesOwnerOnlyFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "esque", "config.json")
	want := &config.Config{Projects: []config.Project{{
		Name:       "blog",
		Dir:        "/srv/blog",
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
	}}}

	if err := config.Save(path, want); err != nil {
		t.Fatalf("Save: %v", err)
	}

	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("config file not written: %v", err)
	}
	if got := info.Mode().Perm(); got != 0o600 {
		t.Errorf("config file mode = %04o, want 0600", got)
	}

	folder, err := os.Stat(filepath.Dir(path))
	if err != nil {
		t.Fatalf("config folder not created: %v", err)
	}
	if got := folder.Mode().Perm(); got != 0o700 {
		t.Errorf("config folder mode = %04o, want 0700", got)
	}

	got, err := config.Load(path)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("Load returned %+v, want %+v", got, want)
	}
}

func TestAddRefusesUsedName(t *testing.T) {
	cfg := &config.Config{}
	blog := config.Project{Name: "blog", Dir: "/srv/blog", DB: "blog.sqlite", Passphrase: "s3cret"}
	if err := cfg.Add(blog); err != nil {
		t.Fatalf("Add blog: %v", err)
	}

	dup := config.Project{Name: "blog", Dir: "/srv/other", DB: "other.sqlite", Passphrase: "other"}
	err := cfg.Add(dup)
	if !errors.Is(err, config.ErrNameTaken) {
		t.Fatalf("Add duplicate: got %v, want config.ErrNameTaken", err)
	}
	if len(cfg.Projects) != 1 {
		t.Errorf("config has %d projects, want 1", len(cfg.Projects))
	}
}

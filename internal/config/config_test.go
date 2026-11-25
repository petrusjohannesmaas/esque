package config_test

import (
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"strings"
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

func TestRenameRefusesUsedName(t *testing.T) {
	cfg := &config.Config{}
	blog := config.Project{Name: "blog", Dir: "/srv/blog", DB: "blog.sqlite", Passphrase: "s3cret"}
	shop := config.Project{Name: "shop", Dir: "/srv/shop", DB: "shop.sqlite", Passphrase: "sh0p"}
	if err := cfg.Add(blog); err != nil {
		t.Fatalf("Add blog: %v", err)
	}
	if err := cfg.Add(shop); err != nil {
		t.Fatalf("Add shop: %v", err)
	}

	err := cfg.Rename("blog", "shop")
	if !errors.Is(err, config.ErrNameTaken) {
		t.Fatalf("Rename blog to shop: got %v, want config.ErrNameTaken", err)
	}
	if cfg.Projects[0].Name != "blog" || cfg.Projects[1].Name != "shop" {
		t.Errorf("names after refused rename = %q, %q, want blog, shop", cfg.Projects[0].Name, cfg.Projects[1].Name)
	}

	if err := cfg.Rename("blog", "journal"); err != nil {
		t.Fatalf("Rename blog to journal: %v", err)
	}
	if cfg.Projects[0].Name != "journal" {
		t.Errorf("name after rename = %q, want journal", cfg.Projects[0].Name)
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

func TestAddRefusesBadName(t *testing.T) {
	cfg := &config.Config{}
	for _, name := range []string{"my blog", "blog/1"} {
		p := config.Project{Name: name, Dir: "/srv/blog", DB: "blog.sqlite", Passphrase: "s3cret"}
		err := cfg.Add(p)
		if err == nil || !strings.Contains(err.Error(), "project name") {
			t.Errorf("Add %q: got %v, want an error naming the project name", name, err)
		}
	}
	if len(cfg.Projects) != 0 {
		t.Fatalf("config has %d projects after bad names, want 0", len(cfg.Projects))
	}

	ok := config.Project{Name: "blog-1.v2_x", Dir: "/srv/blog", DB: "blog.sqlite", Passphrase: "s3cret"}
	if err := cfg.Add(ok); err != nil {
		t.Fatalf("Add blog-1.v2_x: %v", err)
	}
	if len(cfg.Projects) != 1 {
		t.Errorf("config has %d projects, want 1", len(cfg.Projects))
	}
}

package tui_test

import (
	"bytes"
	"database/sql"
	"io"
	"os"
	"path/filepath"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/exp/teatest"
	"github.com/muesli/termenv"
	_ "modernc.org/sqlite"

	"github.com/petrusjohannesmaas/esque/internal/config"
	"github.com/petrusjohannesmaas/esque/internal/tui"
)

func TestMain(m *testing.M) {
	lipgloss.SetColorProfile(termenv.Ascii)
	os.Exit(m.Run())
}

func newApp(t *testing.T) (*teatest.TestModel, string) {
	t.Helper()
	cfgPath := filepath.Join(t.TempDir(), "esque", "config.json")
	model, err := tui.New(cfgPath, tui.Deps{})
	if err != nil {
		t.Fatalf("tui.New: %v", err)
	}
	return teatest.NewTestModel(t, model, teatest.WithInitialTermSize(100, 40)), cfgPath
}

// waitFor drains the program output into seen and waits until cond matches
// what has arrived since the last call.
func waitFor(t *testing.T, tm *teatest.TestModel, seen *bytes.Buffer, cond func([]byte) bool) {
	t.Helper()
	teatest.WaitFor(t, io.TeeReader(tm.Output(), seen), cond, teatest.WithDuration(2*time.Second))
}

func contains(text string) func([]byte) bool {
	return func(b []byte) bool { return bytes.Contains(b, []byte(text)) }
}

func key(r rune) tea.KeyMsg {
	return tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{r}}
}

func special(t tea.KeyType) tea.KeyMsg {
	return tea.KeyMsg{Type: t}
}

func createSQLite(t *testing.T, dir, name string) {
	t.Helper()
	db, err := sql.Open("sqlite", filepath.Join(dir, name))
	if err != nil {
		t.Fatalf("open %s: %v", name, err)
	}
	if _, err := db.Exec("CREATE TABLE sample (id INTEGER PRIMARY KEY)"); err != nil {
		t.Fatalf("create table in %s: %v", name, err)
	}
	if err := db.Close(); err != nil {
		t.Fatalf("close %s: %v", name, err)
	}
}

func addProject(t *testing.T, tm *teatest.TestModel, seen *bytes.Buffer, dir, passphrase string) {
	t.Helper()
	tm.Send(key('a'))
	waitFor(t, tm, seen, contains("Passphrase"))
	tm.Type("blog")
	tm.Send(special(tea.KeyTab))
	tm.Type(dir)
	tm.Send(special(tea.KeyTab))
	tm.Type(passphrase)
	tm.Send(special(tea.KeyEnter))
}

func readConfig(t *testing.T, cfgPath string) *config.Config {
	t.Helper()
	cfg, err := config.Load(cfgPath)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	return cfg
}

func TestProjectViewListsActions(t *testing.T) {
	dir := t.TempDir()
	createSQLite(t, dir, "app.db")
	tm, _ := newApp(t)
	var seen bytes.Buffer

	waitFor(t, tm, &seen, contains("Add project"))
	addProject(t, tm, &seen, dir, "s3cret")
	waitFor(t, tm, &seen, contains("> blog"))
	tm.Send(special(tea.KeyEnter))
	waitFor(t, tm, &seen, contains("Reveal passphrase"))

	for _, action := range []string{"Back up", "List backups", "Restore", "S3 setup", "Reveal passphrase", "Rename", "Remove"} {
		if !bytes.Contains(seen.Bytes(), []byte(action)) {
			t.Errorf("output does not show %q", action)
		}
	}

	if err := tm.Quit(); err != nil {
		t.Fatalf("Quit: %v", err)
	}
	tm.WaitFinished(t, teatest.WithFinalTimeout(2*time.Second))
}

func TestRevealPassphraseUntilKey(t *testing.T) {
	dir := t.TempDir()
	createSQLite(t, dir, "app.db")
	tm, _ := newApp(t)
	var seen bytes.Buffer

	waitFor(t, tm, &seen, contains("Add project"))
	addProject(t, tm, &seen, dir, "s3cret")
	waitFor(t, tm, &seen, contains("> blog"))
	tm.Send(special(tea.KeyEnter))
	waitFor(t, tm, &seen, contains("Reveal passphrase"))
	for i := 0; i < 4; i++ {
		tm.Send(special(tea.KeyDown))
	}
	tm.Send(special(tea.KeyEnter))
	waitFor(t, tm, &seen, contains("s3cret"))

	tm.Send(key('x'))
	var final []byte
	waitFor(t, tm, &seen, func(b []byte) bool {
		if bytes.Contains(b, []byte("Back up")) {
			final = append([]byte(nil), b...)
			return true
		}
		return false
	})
	if bytes.Contains(final, []byte("s3cret")) {
		t.Errorf("passphrase remains visible after a key press")
	}

	if err := tm.Quit(); err != nil {
		t.Fatalf("Quit: %v", err)
	}
	tm.WaitFinished(t, teatest.WithFinalTimeout(2*time.Second))
}

func TestRenameShowsNewName(t *testing.T) {
	dir := t.TempDir()
	createSQLite(t, dir, "app.db")
	tm, cfgPath := newApp(t)
	var seen bytes.Buffer

	waitFor(t, tm, &seen, contains("Add project"))
	addProject(t, tm, &seen, dir, "s3cret")
	waitFor(t, tm, &seen, contains("> blog"))
	tm.Send(special(tea.KeyEnter))
	waitFor(t, tm, &seen, contains("Reveal passphrase"))
	for i := 0; i < 5; i++ {
		tm.Send(special(tea.KeyDown))
	}
	tm.Send(special(tea.KeyEnter))
	waitFor(t, tm, &seen, contains("New name"))
	tm.Type("journal")
	tm.Send(special(tea.KeyEnter))

	var list []byte
	waitFor(t, tm, &seen, func(b []byte) bool {
		if bytes.Contains(b, []byte("> journal")) {
			list = append([]byte(nil), b...)
			return true
		}
		return false
	})
	if bytes.Contains(list, []byte("blog")) {
		t.Errorf("list still shows blog after the rename")
	}

	if err := tm.Quit(); err != nil {
		t.Fatalf("Quit: %v", err)
	}
	tm.WaitFinished(t, teatest.WithFinalTimeout(2*time.Second))

	cfg := readConfig(t, cfgPath)
	if len(cfg.Projects) != 1 {
		t.Fatalf("config has %d projects, want 1", len(cfg.Projects))
	}
	if got := cfg.Projects[0].Name; got != "journal" {
		t.Errorf("saved name = %q, want journal", got)
	}
}

func TestRemoveProjectAfterConfirm(t *testing.T) {
	dir := t.TempDir()
	createSQLite(t, dir, "app.db")
	cfgPath := filepath.Join(t.TempDir(), "esque", "config.json")
	cfg := &config.Config{Projects: []config.Project{{
		Name:       "blog",
		Dir:        dir,
		DB:         "app.db",
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
	if err := config.Save(cfgPath, cfg); err != nil {
		t.Fatalf("Save: %v", err)
	}
	model, err := tui.New(cfgPath, tui.Deps{})
	if err != nil {
		t.Fatalf("tui.New: %v", err)
	}
	tm := teatest.NewTestModel(t, model, teatest.WithInitialTermSize(100, 40))
	var seen bytes.Buffer

	waitFor(t, tm, &seen, contains("> blog"))
	tm.Send(special(tea.KeyEnter))
	waitFor(t, tm, &seen, contains("Reveal passphrase"))
	for i := 0; i < 6; i++ {
		tm.Send(special(tea.KeyDown))
	}
	tm.Send(special(tea.KeyEnter))

	var confirm []byte
	waitFor(t, tm, &seen, func(b []byte) bool {
		if bytes.Contains(b, []byte("Remove blog?")) {
			confirm = append([]byte(nil), b...)
			return true
		}
		return false
	})
	if !bytes.Contains(confirm, []byte("passphrase and S3 setup are deleted too")) {
		t.Errorf("confirmation does not say the passphrase is deleted too: %q", confirm)
	}

	tm.Send(key('y'))
	var list []byte
	waitFor(t, tm, &seen, func(b []byte) bool {
		if bytes.Contains(b, []byte("Add project")) {
			list = append([]byte(nil), b...)
			return true
		}
		return false
	})
	if bytes.Contains(list, []byte("blog")) {
		t.Errorf("list still shows blog after the removal")
	}

	if err := tm.Quit(); err != nil {
		t.Fatalf("Quit: %v", err)
	}
	tm.WaitFinished(t, teatest.WithFinalTimeout(2*time.Second))

	saved := readConfig(t, cfgPath)
	if len(saved.Projects) != 0 {
		t.Fatalf("config has %d projects, want none", len(saved.Projects))
	}
	raw, err := os.ReadFile(cfgPath)
	if err != nil {
		t.Fatalf("read config: %v", err)
	}
	for _, secret := range []string{"s3cret", "secret"} {
		if bytes.Contains(raw, []byte(secret)) {
			t.Errorf("config still holds %q after the removal", secret)
		}
	}
	if _, err := os.Stat(filepath.Join(dir, "app.db")); err != nil {
		t.Errorf("database file is gone after the removal: %v", err)
	}
}

func TestAddProjectAppearsInList(t *testing.T) {
	dir := t.TempDir()
	createSQLite(t, dir, "app.db")
	tm, cfgPath := newApp(t)
	var seen bytes.Buffer

	waitFor(t, tm, &seen, contains("Add project"))
	addProject(t, tm, &seen, dir, "s3cret")
	waitFor(t, tm, &seen, contains("> blog"))

	if err := tm.Quit(); err != nil {
		t.Fatalf("Quit: %v", err)
	}
	tm.WaitFinished(t, teatest.WithFinalTimeout(2*time.Second))

	cfg := readConfig(t, cfgPath)
	if len(cfg.Projects) != 1 {
		t.Fatalf("config has %d projects, want 1", len(cfg.Projects))
	}
	got := cfg.Projects[0]
	if got.Name != "blog" || got.DB != "app.db" {
		t.Errorf("saved project = %+v, want name blog and db app.db", got)
	}
	if bytes.Contains(seen.Bytes(), []byte("s3cret")) {
		t.Errorf("program output contains the passphrase")
	}
}

func TestAddProjectPicksAmongSeveralFiles(t *testing.T) {
	dir := t.TempDir()
	createSQLite(t, dir, "a.sqlite")
	createSQLite(t, dir, "b.sqlite")
	tm, cfgPath := newApp(t)
	var seen bytes.Buffer

	waitFor(t, tm, &seen, contains("Add project"))
	addProject(t, tm, &seen, dir, "s3cret")
	waitFor(t, tm, &seen, contains("Choose the SQLite database file"))
	tm.Send(special(tea.KeyDown))
	tm.Send(special(tea.KeyEnter))
	waitFor(t, tm, &seen, contains("> blog"))

	if err := tm.Quit(); err != nil {
		t.Fatalf("Quit: %v", err)
	}
	tm.WaitFinished(t, teatest.WithFinalTimeout(2*time.Second))

	cfg := readConfig(t, cfgPath)
	if len(cfg.Projects) != 1 {
		t.Fatalf("config has %d projects, want 1", len(cfg.Projects))
	}
	if got := cfg.Projects[0].DB; got != "b.sqlite" {
		t.Errorf("saved db = %q, want b.sqlite", got)
	}
}

func TestAddProjectWithNoSQLiteFile(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "notes.txt"), []byte("just notes"), 0o600); err != nil {
		t.Fatal(err)
	}
	tm, cfgPath := newApp(t)
	var seen bytes.Buffer

	waitFor(t, tm, &seen, contains("Add project"))
	addProject(t, tm, &seen, dir, "s3cret")
	waitFor(t, tm, &seen, func(b []byte) bool {
		return bytes.Contains(b, []byte("No SQLite database found in "+dir)) ||
			bytes.Contains(b, []byte("> blog"))
	})

	if err := tm.Quit(); err != nil {
		t.Fatalf("Quit: %v", err)
	}
	tm.WaitFinished(t, teatest.WithFinalTimeout(2*time.Second))

	cfg := readConfig(t, cfgPath)
	if len(cfg.Projects) != 0 {
		t.Errorf("config has %d projects, want none", len(cfg.Projects))
	}
	if !bytes.Contains(seen.Bytes(), []byte("No SQLite database found in "+dir)) {
		t.Errorf("output does not show the missing database message")
	}
}

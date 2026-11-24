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

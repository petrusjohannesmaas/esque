package tui

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/spinner"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/dustin/go-humanize"

	"github.com/petrusjohannesmaas/esque/internal/backup"
	"github.com/petrusjohannesmaas/esque/internal/config"
	"github.com/petrusjohannesmaas/esque/internal/remote"
)

const (
	backupNaming = iota
	backupWorking
	backupDone
)

// backupDoneMsg carries the result of a backup run.
type backupDoneMsg struct {
	object remote.Object
	err    error
}

// backupScreen asks for a name and runs the backup while a spinner turns.
type backupScreen struct {
	m      *Model
	index  int
	form   fieldSet
	spin   spinner.Model
	phase  int
	object remote.Object
	height int
}

func newBackupScreen(m *Model, index int) *backupScreen {
	p := m.cfg.Projects[index]
	f := newField("Backup name", "backup name", "")
	f.input.SetValue(backup.DefaultName(p.Name, m.now()))
	s := &backupScreen{m: m, index: index, form: fieldSet{fields: []field{f}}}
	s.spin = spinner.New(spinner.WithSpinner(spinner.Dot), spinner.WithStyle(accentStyle))
	return s
}

// location names the bucket and prefix of a setup, such as
// backups/blog/.
func location(setup config.S3Setup) string {
	prefix := strings.Trim(setup.Prefix, "/")
	if prefix == "" {
		return setup.Bucket
	}
	return setup.Bucket + "/" + prefix + "/"
}

func (s *backupScreen) init() tea.Cmd { return s.form.focusOn(0) }

func (s *backupScreen) setSize(width, height int) {
	s.form.setWidth(width)
	s.height = height
}

func (s *backupScreen) crumbs() []string {
	return []string{"Projects", s.m.cfg.Projects[s.index].Name, "Back up"}
}

func (s *backupScreen) typing() bool { return s.phase == backupNaming }

func (s *backupScreen) keys() keyMap {
	switch s.phase {
	case backupWorking:
		return keyMap{short: []key.Binding{keyForceQuit}}
	case backupDone:
		return keyMap{short: []key.Binding{keyBack}}
	}
	return keyMap{short: []key.Binding{backupKeys.Start, keyBack}}
}

func (s *backupScreen) update(msg tea.Msg) (screen, tea.Cmd) {
	switch msg := msg.(type) {
	case backupDoneMsg:
		if msg.err != nil {
			s.form.fields[0].err = describe(msg.err, s.m.endpoint(s.index))
			s.phase = backupNaming
			return s, s.form.focusOn(0)
		}
		s.object = msg.object
		s.phase = backupDone
		s.m.forget(s.m.cfg.Projects[s.index].Name)
		return s, nil
	case spinner.TickMsg:
		if s.phase != backupWorking {
			return s, nil
		}
		var cmd tea.Cmd
		s.spin, cmd = s.spin.Update(msg)
		return s, cmd
	case tea.KeyMsg:
		switch s.phase {
		case backupWorking:
			return s, nil
		case backupDone:
			if key.Matches(msg, keyBack) {
				return newProjectView(s.m, s.index), nil
			}
			return s, nil
		}
		switch {
		case key.Matches(msg, keyBack):
			return newProjectView(s.m, s.index), nil
		case key.Matches(msg, backupKeys.Start):
			return s, s.start(s.form.fields[0].value())
		}
	}
	return s, s.form.updateFocused(msg)
}

func (s *backupScreen) start(name string) tea.Cmd {
	project := s.m.cfg.Projects[s.index]
	s.form.clearErrors()
	store, err := s.m.deps.OpenStore(*project.S3)
	if err != nil {
		s.form.fields[0].err = describe(err, s.m.endpoint(s.index))
		return nil
	}
	s.phase = backupWorking
	s.form.fields[0].blur()
	ctx := context.Background()
	return tea.Batch(s.spin.Tick, func() tea.Msg {
		object, err := backup.Backup(ctx, project, name, store)
		return backupDoneMsg{object: object, err: err}
	})
}

// describe turns a backup, list or restore failure into a message for the
// screen.
func describe(err error, endpoint string) string {
	switch {
	case errors.Is(err, remote.ErrAccessDenied):
		return "S3 refused the access key"
	case errors.Is(err, remote.ErrUnreachable):
		return "Cannot reach S3 at " + endpoint
	case errors.Is(err, remote.ErrNotFound):
		return "S3 object not found"
	case errors.Is(err, backup.ErrBadBackup):
		return backup.ErrBadBackup.Error()
	}
	return err.Error()
}

func (s *backupScreen) view() string {
	project := s.m.cfg.Projects[s.index]
	width := s.form.width
	switch s.phase {
	case backupWorking:
		return s.spin.View() + " " + wrap("Uploading…", width-2) + "\n\n" +
			mutedStyle.Render(truncate(s.form.fields[0].value(), width))
	case backupDone:
		rows := [][2]string{
			{"Name", s.object.Name},
			{"Size", humanize.Bytes(uint64(s.object.Size))},
			{"Where", location(*project.S3)},
		}
		var b strings.Builder
		b.WriteString(okStyle.Render(okGlyph+" Backup uploaded") + "\n\n")
		for _, r := range rows {
			value := strings.TrimLeft(indent(wrap(r[1], width-7), 7), " ")
			b.WriteString(mutedStyle.Render(fmt.Sprintf("%-6s", r[0])) + " " + value + "\n")
		}
		return b.String()
	}
	return s.form.view(s.height)
}

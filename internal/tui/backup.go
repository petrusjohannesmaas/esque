package tui

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"

	"github.com/petrusjohannesmaas/esque/internal/backup"
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
	input  textinput.Model
	spin   spinner.Model
	phase  int
	status string
	object remote.Object
}

func newBackupScreen(m *Model, index int) *backupScreen {
	s := &backupScreen{m: m, index: index}
	s.input = textinput.New()
	s.input.Width = 60
	s.input.Placeholder = "backup name"
	s.input.SetValue(backup.DefaultName(m.cfg.Projects[index].Name, m.now()))
	s.input.Focus()
	s.spin = spinner.New()
	return s
}

func (s *backupScreen) update(msg tea.Msg) (screen, tea.Cmd) {
	if done, ok := msg.(backupDoneMsg); ok {
		if done.err != nil {
			s.status = describe(done.err, s.m.endpoint(s.index))
			s.phase = backupNaming
			s.input.Focus()
			return s, textinput.Blink
		}
		s.object = done.object
		s.phase = backupDone
		return s, nil
	}
	key, ok := msg.(tea.KeyMsg)
	if !ok {
		var cmd tea.Cmd
		s.spin, cmd = s.spin.Update(msg)
		return s, cmd
	}
	if s.phase == backupWorking {
		return s, nil
	}
	switch key.String() {
	case "esc":
		return newProjectView(s.m, s.index), nil
	case "enter":
		if s.phase != backupNaming {
			return s, nil
		}
		return s, s.start(strings.TrimSpace(s.input.Value()))
	}
	var cmd tea.Cmd
	s.input, cmd = s.input.Update(key)
	return s, cmd
}

func (s *backupScreen) start(name string) tea.Cmd {
	project := s.m.cfg.Projects[s.index]
	store, err := s.m.deps.OpenStore(*project.S3)
	if err != nil {
		s.status = describe(err, s.m.endpoint(s.index))
		return nil
	}
	s.phase = backupWorking
	s.status = ""
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
	var b strings.Builder
	fmt.Fprintf(&b, "Back up %s\n\n", project.Name)
	switch s.phase {
	case backupWorking:
		fmt.Fprintf(&b, "%s Uploading %s\n", s.spin.View(), s.input.Value())
	case backupDone:
		fmt.Fprintf(&b, "Backup uploaded\n\n%s\n%d bytes\n", s.object.Name, s.object.Size)
		b.WriteString("\nesc back\n")
	default:
		fmt.Fprintf(&b, "Name\n%s\n", s.input.View())
		if s.status != "" {
			fmt.Fprintf(&b, "\n%s\n", s.status)
		}
		b.WriteString("\nenter back up • esc back\n")
	}
	return b.String()
}

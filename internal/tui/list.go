package tui

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/dustin/go-humanize"

	"github.com/petrusjohannesmaas/esque/internal/backup"
	"github.com/petrusjohannesmaas/esque/internal/remote"
)

const (
	listLoading = iota
	listBrowsing
	listFolder
	listRestoring
	listRestored
)

// listDoneMsg carries the result of listing a project's backups.
type listDoneMsg struct {
	objects []remote.Object
	err     error
}

// restoreDoneMsg carries the result of a restore run.
type restoreDoneMsg struct {
	path string
	err  error
}

// backupList shows a project's backups and restores one into a folder.
type backupList struct {
	m       *Model
	index   int
	phase   int
	objects []remote.Object
	cursor  int
	input   textinput.Model
	status  string
	path    string
}

func newBackupList(m *Model, index int) (*backupList, tea.Cmd) {
	s := &backupList{m: m, index: index, phase: listLoading}
	store, err := m.deps.OpenStore(*m.cfg.Projects[index].S3)
	if err != nil {
		s.phase = listBrowsing
		s.status = describe(err, m.endpoint(index))
		return s, nil
	}
	ctx := context.Background()
	return s, func() tea.Msg {
		objects, err := backup.List(ctx, store)
		return listDoneMsg{objects: objects, err: err}
	}
}

func (s *backupList) update(msg tea.Msg) (screen, tea.Cmd) {
	switch msg := msg.(type) {
	case listDoneMsg:
		s.phase = listBrowsing
		if msg.err != nil {
			s.status = describe(msg.err, s.m.endpoint(s.index))
			return s, nil
		}
		s.objects = msg.objects
		return s, nil
	case restoreDoneMsg:
		if msg.err != nil {
			s.phase = listFolder
			s.status = describe(msg.err, s.m.endpoint(s.index))
			s.input.Focus()
			return s, textinput.Blink
		}
		s.path = msg.path
		s.phase = listRestored
		return s, nil
	}
	key, ok := msg.(tea.KeyMsg)
	if !ok {
		if s.phase == listFolder {
			var cmd tea.Cmd
			s.input, cmd = s.input.Update(msg)
			return s, cmd
		}
		return s, nil
	}
	switch s.phase {
	case listFolder:
		return s.updateFolder(key)
	case listRestored:
		if key.String() == "esc" {
			return newProjectView(s.m, s.index), nil
		}
		return s, nil
	case listLoading, listRestoring:
		return s, nil
	}
	switch key.String() {
	case "esc":
		return newProjectView(s.m, s.index), nil
	case "up", "k":
		if s.cursor > 0 {
			s.cursor--
		}
	case "down", "j":
		if s.cursor < len(s.objects)-1 {
			s.cursor++
		}
	case "enter":
		if len(s.objects) == 0 {
			return s, nil
		}
		s.startFolder()
	}
	return s, nil
}

func (s *backupList) startFolder() {
	s.input = textinput.New()
	s.input.Width = 80
	s.input.Placeholder = "folder"
	s.input.SetValue(filepath.Join(s.m.cfg.Projects[s.index].Dir, "restored"))
	s.input.Focus()
	s.status = ""
	s.phase = listFolder
}

func (s *backupList) updateFolder(key tea.KeyMsg) (screen, tea.Cmd) {
	switch key.String() {
	case "esc":
		s.phase = listBrowsing
		return s, nil
	case "enter":
		name := strings.TrimSuffix(s.objects[s.cursor].Name, ".sqlite.enc")
		dest := strings.TrimSpace(s.input.Value())
		store, err := s.m.deps.OpenStore(*s.m.cfg.Projects[s.index].S3)
		if err != nil {
			s.status = describe(err, s.m.endpoint(s.index))
			return s, nil
		}
		s.phase = listRestoring
		s.status = ""
		project := s.m.cfg.Projects[s.index]
		ctx := context.Background()
		return s, func() tea.Msg {
			path, err := backup.Restore(ctx, project, name, dest, store)
			return restoreDoneMsg{path: path, err: err}
		}
	}
	var cmd tea.Cmd
	s.input, cmd = s.input.Update(key)
	return s, cmd
}

func (s *backupList) view() string {
	project := s.m.cfg.Projects[s.index]
	var b strings.Builder
	fmt.Fprintf(&b, "Backups for %s\n\n", project.Name)
	name := ""
	if s.cursor < len(s.objects) {
		name = strings.TrimSuffix(s.objects[s.cursor].Name, ".sqlite.enc")
	}
	switch s.phase {
	case listLoading:
		b.WriteString("Listing backups\n")
		return b.String()
	case listRestoring:
		fmt.Fprintf(&b, "Restoring %s\n", name)
		return b.String()
	case listFolder:
		fmt.Fprintf(&b, "Restore %s\n\nFolder\n%s\n", name, s.input.View())
		if s.status != "" {
			fmt.Fprintf(&b, "\n%s\n", s.status)
		}
		b.WriteString("\nenter restore • esc back\n")
		return b.String()
	case listRestored:
		fmt.Fprintf(&b, "Restored %s\n", s.path)
		b.WriteString("\nesc back\n")
		return b.String()
	}
	if s.status != "" {
		fmt.Fprintf(&b, "%s\n", s.status)
	} else if len(s.objects) == 0 {
		b.WriteString("No backups yet\n")
	} else {
		for i, obj := range s.objects {
			fmt.Fprintf(&b, "%s %s  %s  %s\n", cursorMark(i == s.cursor), obj.Name,
				humanize.Bytes(uint64(obj.Size)), obj.Modified.Local().Format("2 January 2006 15:04"))
		}
	}
	b.WriteString("\nesc back • enter restore\n")
	return b.String()
}

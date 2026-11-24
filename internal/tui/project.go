package tui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"

	"github.com/petrusjohannesmaas/esque/internal/config"
)

// projectActionNames lists the project view's actions, in display order.
var projectActionNames = []string{
	"Back up",
	"List backups",
	"Restore",
	"S3 setup",
	"Reveal passphrase",
	"Rename",
	"Remove",
}

const (
	actionBackup = iota
	actionListBackups
	actionRestore
	actionS3
	actionReveal
	actionRename
	actionRemove
)

const (
	projectActions = iota
	projectReveal
	projectRename
	projectRemove
)

type projectView struct {
	m      *Model
	index  int
	cursor int
	mode   int
	input  textinput.Model
	status string
}

func newProjectView(m *Model, index int) *projectView {
	return &projectView{m: m, index: index}
}

func (s *projectView) update(msg tea.Msg) (screen, tea.Cmd) {
	key, ok := msg.(tea.KeyMsg)
	if !ok {
		return s, nil
	}
	switch s.mode {
	case projectReveal:
		s.mode = projectActions
		return s, nil
	case projectRename:
		return s.updateRename(key)
	case projectRemove:
		return s.updateRemove(key)
	}
	switch key.String() {
	case "esc":
		return newMenu(s.m), nil
	case "up", "k":
		if s.cursor > 0 {
			s.cursor--
		}
	case "down", "j":
		if s.cursor < len(projectActionNames)-1 {
			s.cursor++
		}
	case "enter":
		switch s.cursor {
		case actionReveal:
			s.mode = projectReveal
		case actionRename:
			s.startRename()
		case actionRemove:
			s.mode = projectRemove
		}
	}
	return s, nil
}

func (s *projectView) startRename() {
	s.input = textinput.New()
	s.input.Width = 60
	s.input.Placeholder = s.m.cfg.Projects[s.index].Name
	s.input.Focus()
	s.status = ""
	s.mode = projectRename
}

func (s *projectView) updateRename(key tea.KeyMsg) (screen, tea.Cmd) {
	switch key.String() {
	case "esc":
		s.input.Blur()
		s.mode = projectActions
		return s, nil
	case "enter":
		name := strings.TrimSpace(s.input.Value())
		if err := s.m.cfg.Rename(s.m.cfg.Projects[s.index].Name, name); err != nil {
			s.status = err.Error()
			return s, nil
		}
		if err := config.Save(s.m.cfgPath, s.m.cfg); err != nil {
			s.status = err.Error()
			return s, nil
		}
		return newMenu(s.m), nil
	}
	var cmd tea.Cmd
	s.input, cmd = s.input.Update(key)
	return s, cmd
}

func (s *projectView) updateRemove(key tea.KeyMsg) (screen, tea.Cmd) {
	switch key.String() {
	case "y":
		name := s.m.cfg.Projects[s.index].Name
		if err := s.m.cfg.Remove(name); err != nil {
			s.status = err.Error()
			s.mode = projectActions
			return s, nil
		}
		if err := config.Save(s.m.cfgPath, s.m.cfg); err != nil {
			s.status = err.Error()
			s.mode = projectActions
			return s, nil
		}
		return newMenu(s.m), nil
	case "n", "esc":
		s.mode = projectActions
	}
	return s, nil
}

func (s *projectView) view() string {
	p := s.m.cfg.Projects[s.index]
	var b strings.Builder
	switch s.mode {
	case projectReveal:
		fmt.Fprintf(&b, "Passphrase for %s\n\n%s\n\npress any key to hide\n", p.Name, p.Passphrase)
		return b.String()
	case projectRename:
		fmt.Fprintf(&b, "Rename %s\n\nNew name\n%s\n", p.Name, s.input.View())
		if s.status != "" {
			fmt.Fprintf(&b, "\n%s\n", s.status)
		}
		b.WriteString("\nenter save • esc cancel\n")
		return b.String()
	case projectRemove:
		fmt.Fprintf(&b, "Remove %s? Its passphrase and S3 setup are deleted too. The database file and backups in S3 stay. (y/n)\n", p.Name)
		return b.String()
	}
	fmt.Fprintf(&b, "%s\n\n", p.Name)
	for i, action := range projectActionNames {
		fmt.Fprintf(&b, "%s %s\n", cursorMark(i == s.cursor), action)
	}
	b.WriteString("\nesc back • enter choose\n")
	return b.String()
}

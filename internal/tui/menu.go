package tui

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
)

type menu struct {
	m      *Model
	cursor int
}

func newMenu(m *Model) *menu {
	return &menu{m: m}
}

func (s *menu) update(msg tea.Msg) (screen, tea.Cmd) {
	key, ok := msg.(tea.KeyMsg)
	if !ok {
		return s, nil
	}
	last := len(s.m.cfg.Projects)
	switch key.String() {
	case "q":
		return s, tea.Quit
	case "up", "k":
		if s.cursor > 0 {
			s.cursor--
		}
	case "down", "j":
		if s.cursor < last {
			s.cursor++
		}
	case "a":
		return newAddProject(s.m), nil
	case "enter":
		if s.cursor == last {
			return newAddProject(s.m), nil
		}
		return newProjectView(s.m, s.cursor), nil
	}
	return s, nil
}

func (s *menu) view() string {
	var b strings.Builder
	b.WriteString("Esque\n\nProjects\n")
	if len(s.m.cfg.Projects) == 0 {
		b.WriteString("  (none)\n")
	}
	for i, p := range s.m.cfg.Projects {
		fmt.Fprintf(&b, "%s %s\n", cursorMark(i == s.cursor), p.Name)
	}
	fmt.Fprintf(&b, "%s Add project\n", cursorMark(s.cursor == len(s.m.cfg.Projects)))
	b.WriteString("\na add project • enter open • q quit\n")
	return b.String()
}

func cursorMark(on bool) string {
	if on {
		return ">"
	}
	return " "
}

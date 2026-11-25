package tui

import (
	"github.com/charmbracelet/bubbles/key"
	tea "github.com/charmbracelet/bubbletea"

	"github.com/petrusjohannesmaas/esque/internal/config"
)

// rename asks for a project's new name.
type rename struct {
	m      *Model
	index  int
	form   fieldSet
	height int
}

func newRename(m *Model, index int) *rename {
	name := m.cfg.Projects[index].Name
	f := newField("New name", name, "Letters, digits, -, _ and . only.")
	return &rename{m: m, index: index, form: fieldSet{fields: []field{f}}}
}

func (s *rename) init() tea.Cmd { return s.form.focusOn(0) }

func (s *rename) setSize(width, height int) {
	s.form.setWidth(width)
	s.height = height
}

func (s *rename) crumbs() []string {
	return []string{"Projects", s.m.cfg.Projects[s.index].Name, "Rename"}
}

func (s *rename) typing() bool { return true }

func (s *rename) keys() keyMap { return keyMap{short: []key.Binding{formKeys.Save, keyCancel}} }

func (s *rename) update(msg tea.Msg) (screen, tea.Cmd) {
	if k, ok := msg.(tea.KeyMsg); ok {
		switch {
		case key.Matches(k, keyCancel):
			return newProjectView(s.m, s.index), nil
		case key.Matches(k, formKeys.Save):
			return s.save()
		}
	}
	return s, s.form.updateFocused(msg)
}

func (s *rename) save() (screen, tea.Cmd) {
	f := &s.form.fields[0]
	name := f.value()
	old := s.m.cfg.Projects[s.index].Name
	if err := s.m.change(func(cfg *config.Config) error { return cfg.Rename(old, name) }); err != nil {
		f.err = err.Error()
		return s, nil
	}
	s.m.forget(old)
	return newProjects(s.m), nil
}

func (s *rename) view() string { return s.form.view(s.height) }

package tui

import (
	"path/filepath"
	"strings"

	"github.com/charmbracelet/bubbles/key"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/petrusjohannesmaas/esque/internal/config"
)

const (
	actionBackup = iota
	actionListBackups
	actionRestore
	actionS3
	actionReveal
	actionRename
	actionRemove
	actionCount
)

// projectActionNames lists the project menu's actions, in display order.
var projectActionNames = [actionCount]string{
	"Back up",
	"List backups",
	"Restore",
	"S3 setup",
	"Reveal passphrase",
	"Rename",
	"Remove",
}

// groupStarts holds the actions that begin a new group, so the menu puts a
// blank line above them.
var groupStarts = map[int]bool{actionS3: true, actionRename: true}

// needsS3 reports whether an action cannot run without an S3 setup.
func needsS3(action int) bool {
	return action == actionBackup || action == actionListBackups || action == actionRestore
}

// projectView is the menu of actions for one project.
type projectView struct {
	m        *Model
	index    int
	cursor   int
	flash    flash
	pending  *flash
	removing *confirm
	width    int
	height   int
}

func newProjectView(m *Model, index int) *projectView {
	return &projectView{m: m, index: index}
}

// withFlash shows a status message once the menu opens.
func (s *projectView) withFlash(kind flashKind, text string) *projectView {
	s.pending = &flash{kind: kind, text: text}
	return s
}

func (s *projectView) init() tea.Cmd {
	if s.pending == nil {
		return nil
	}
	p := s.pending
	s.pending = nil
	return s.flash.set(s.m, p.kind, p.text)
}

func (s *projectView) setSize(width, height int) { s.width, s.height = width, height }

func (s *projectView) project() config.Project { return s.m.cfg.Projects[s.index] }

func (s *projectView) crumbs() []string { return []string{"Projects", s.project().Name} }

func (s *projectView) typing() bool { return false }

func (s *projectView) keys() keyMap {
	if s.removing != nil {
		return s.removing.keys()
	}
	return keyMap{short: []key.Binding{keyMove, menuKeys.Choose, keyBack}}
}

func (s *projectView) update(msg tea.Msg) (screen, tea.Cmd) {
	if expired, ok := msg.(flashExpiredMsg); ok {
		s.flash.expire(expired)
		return s, nil
	}
	k, ok := msg.(tea.KeyMsg)
	if !ok {
		return s, nil
	}
	if s.removing != nil {
		return s.updateRemove(k)
	}
	switch {
	case key.Matches(k, keyBack):
		return newProjects(s.m), nil
	case key.Matches(k, keyUp):
		if s.cursor > 0 {
			s.cursor--
		}
	case key.Matches(k, keyDown):
		if s.cursor < actionCount-1 {
			s.cursor++
		}
	case key.Matches(k, menuKeys.Choose):
		return s.choose()
	}
	return s, nil
}

func (s *projectView) choose() (screen, tea.Cmd) {
	if needsS3(s.cursor) && s.project().S3 == nil {
		return s, s.flash.set(s.m, flashWarn, warnGlyph+" Set up S3 first")
	}
	switch s.cursor {
	case actionBackup:
		return newBackupScreen(s.m, s.index), nil
	case actionListBackups:
		return newBackupList(s.m, s.index, false), nil
	case actionRestore:
		return newBackupList(s.m, s.index, true), nil
	case actionS3:
		return newS3Form(s.m, s.index), nil
	case actionReveal:
		return newReveal(s.m, s.index), nil
	case actionRename:
		return newRename(s.m, s.index), nil
	case actionRemove:
		name := s.project().Name
		c := newConfirm(
			"Remove "+name+"?",
			"Forgets its passphrase and S3 setup. The database and backups stay.",
			"remove", true, false)
		s.removing = &c
	}
	return s, nil
}

func (s *projectView) updateRemove(k tea.KeyMsg) (screen, tea.Cmd) {
	done, yes := s.removing.answer(k)
	if !done {
		return s, nil
	}
	s.removing = nil
	if !yes {
		return s, nil
	}
	name := s.project().Name
	if err := s.m.change(func(cfg *config.Config) error { return cfg.Remove(name) }); err != nil {
		return s, s.flash.set(s.m, flashErr, err.Error())
	}
	s.m.forget(name)
	return newProjects(s.m), nil
}

func (s *projectView) view() string {
	if s.removing != nil {
		return s.removing.view(s.width, s.height)
	}
	p := s.project()
	noS3 := p.S3 == nil
	var b strings.Builder
	b.WriteString(titleStyle.Render(truncate(p.Name, s.width)) + "\n")
	b.WriteString(mutedStyle.Render(truncate(filepath.Join(p.Dir, p.DB), s.width)) + "\n")
	if noS3 {
		b.WriteString(warnStyle.Render(wrap(warnGlyph+" S3 not set up", s.width)) + "\n")
	}
	b.WriteString("\n")
	for i, name := range projectActionNames {
		if groupStarts[i] {
			b.WriteString("\n")
		}
		selected := i == s.cursor
		style := lipgloss.NewStyle()
		switch {
		case i == actionRemove:
			style = dangerStyle
		case noS3 && needsS3(i):
			style = dimStyle
		case selected:
			style = selectedStyle
		}
		if selected {
			style = style.Bold(true)
		}
		line := marker(selected) + style.Render(name)
		if i == actionS3 && noS3 {
			line += " " + warnStyle.Render(warnGlyph)
		}
		b.WriteString(line + "\n")
	}
	if f := s.flash.view(s.width); f != "" {
		b.WriteString("\n" + f)
	}
	return b.String()
}

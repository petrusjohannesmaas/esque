package tui

import (
	"fmt"
	"time"

	"github.com/charmbracelet/bubbles/key"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// revealSeconds is how long the passphrase stays on screen.
const revealSeconds = 30

// revealTickMsg counts down one second of the reveal with the same id.
type revealTickMsg struct{ id int }

// reveal shows a project's passphrase until a key is pressed or the
// countdown ends.
type reveal struct {
	m      *Model
	index  int
	pass   string
	left   int
	id     int
	width  int
	height int
}

func newReveal(m *Model, index int) *reveal {
	return &reveal{m: m, index: index, pass: m.cfg.Projects[index].Passphrase, left: revealSeconds}
}

func (s *reveal) init() tea.Cmd {
	s.id = s.m.nextID()
	return s.tick()
}

func (s *reveal) tick() tea.Cmd {
	id := s.id
	return tea.Tick(time.Second, func(time.Time) tea.Msg { return revealTickMsg{id: id} })
}

func (s *reveal) setSize(width, height int) { s.width, s.height = width, height }

func (s *reveal) crumbs() []string {
	return []string{"Projects", s.m.cfg.Projects[s.index].Name, "Passphrase"}
}

// typing is true so that every key, ? included, hides the passphrase.
func (s *reveal) typing() bool { return true }

func (s *reveal) keys() keyMap { return keyMap{short: []key.Binding{revealKeys.Hide}} }

func (s *reveal) update(msg tea.Msg) (screen, tea.Cmd) {
	switch msg := msg.(type) {
	case revealTickMsg:
		if msg.id != s.id {
			return s, nil
		}
		s.left--
		if s.left <= 0 {
			return s.leave(), nil
		}
		return s, s.tick()
	case tea.KeyMsg:
		return s.leave(), nil
	}
	return s, nil
}

// leave drops the passphrase from this screen and returns to the menu.
func (s *reveal) leave() screen {
	s.pass = ""
	s.id = 0
	return newProjectView(s.m, s.index)
}

func (s *reveal) view() string {
	warning := warnStyle.Render(wrap(warnGlyph+" Keep this private.", s.width))
	box := secretStyle.Render(s.pass)
	countdown := mutedStyle.Render(fmt.Sprintf("hiding in %ds", s.left))
	rest := max(s.height-lipgloss.Height(warning)-1, lipgloss.Height(box)+2)
	middle := lipgloss.Place(s.width, rest, lipgloss.Center, lipgloss.Center,
		lipgloss.JoinVertical(lipgloss.Center, box, "", countdown))
	return warning + "\n" + middle
}

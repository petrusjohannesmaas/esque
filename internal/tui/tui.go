package tui

import (
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/petrusjohannesmaas/esque/internal/config"
	"github.com/petrusjohannesmaas/esque/internal/remote"
)

// Deps holds what the screens need from outside.
type Deps struct {
	OpenStore func(config.S3Setup) (remote.Store, error)
	Now       func() time.Time
}

// Model is the root of the terminal interface.
type Model struct {
	cfgPath string
	deps    Deps
	cfg     *config.Config
	screen  screen
	width   int
	height  int
}

// screen is one page of the interface.
type screen interface {
	update(tea.Msg) (screen, tea.Cmd)
	view() string
}

// New loads the config at cfgPath and returns the main menu.
func New(cfgPath string, deps Deps) (*Model, error) {
	cfg, err := config.Load(cfgPath)
	if err != nil {
		return nil, err
	}
	m := &Model{cfgPath: cfgPath, deps: deps, cfg: cfg}
	m.screen = newMenu(m)
	return m, nil
}

func (m *Model) Init() tea.Cmd { return nil }

func (m *Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	if size, ok := msg.(tea.WindowSizeMsg); ok {
		m.width, m.height = size.Width, size.Height
	}
	if key, ok := msg.(tea.KeyMsg); ok && key.String() == "ctrl+c" {
		return m, tea.Quit
	}
	next, cmd := m.screen.update(msg)
	m.screen = next
	return m, cmd
}

func (m *Model) View() string {
	return m.screen.view()
}

// now returns the current local time, or the fixed clock the tests pass.
func (m *Model) now() time.Time {
	if m.deps.Now != nil {
		return m.deps.Now()
	}
	return time.Now()
}

// endpoint returns the project's S3 endpoint, or "" without a setup.
func (m *Model) endpoint(index int) string {
	if setup := m.cfg.Projects[index].S3; setup != nil {
		return setup.Endpoint
	}
	return ""
}

// addProject saves a new project and returns to the menu.
func (m *Model) addProject(p config.Project) error {
	return m.change(func(cfg *config.Config) error { return cfg.Add(p) })
}

// change applies fn to a copy of the config and saves the copy. The model
// keeps the copy only when both succeed, so a failed save leaves the config in
// memory as it is on disk.
func (m *Model) change(fn func(*config.Config) error) error {
	next := m.cfg.Clone()
	if err := fn(next); err != nil {
		return err
	}
	if err := config.Save(m.cfgPath, next); err != nil {
		return err
	}
	m.cfg = next
	return nil
}

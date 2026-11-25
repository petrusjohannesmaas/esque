package tui

import (
	"context"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/help"
	"github.com/charmbracelet/bubbles/key"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/petrusjohannesmaas/esque/internal/backup"
	"github.com/petrusjohannesmaas/esque/internal/config"
	"github.com/petrusjohannesmaas/esque/internal/remote"
)

// Deps holds what the screens need from outside.
type Deps struct {
	OpenStore func(config.S3Setup) (remote.Store, error)
	Now       func() time.Time
}

// Model is the root of the terminal interface. It draws the frame, header and
// footer, and routes messages to the current screen.
type Model struct {
	cfgPath string
	deps    Deps
	cfg     *config.Config
	screen  screen
	width   int
	height  int
	help    help.Model
	// info holds what the projects screen knows about each project's
	// backups, by project name.
	info map[string]projectInfo
	seq  int
}

// screen is one page of the interface. The root model calls init when the
// screen becomes current and setSize whenever the space for content changes.
type screen interface {
	init() tea.Cmd
	update(tea.Msg) (screen, tea.Cmd)
	view() string
	setSize(width, height int)
	// crumbs is the breadcrumb path shown in the header.
	crumbs() []string
	// keys lists the bindings the footer shows.
	keys() keyMap
	// typing reports whether a text field takes every key, so the root does
	// not treat ? as the help toggle.
	typing() bool
}

// New loads the config at cfgPath and returns the projects screen.
func New(cfgPath string, deps Deps) (*Model, error) {
	cfg, err := config.Load(cfgPath)
	if err != nil {
		return nil, err
	}
	m := &Model{cfgPath: cfgPath, deps: deps, cfg: cfg, info: map[string]projectInfo{}}
	m.help = newHelp()
	m.screen = newProjects(m)
	return m, nil
}

func newHelp() help.Model {
	h := help.New()
	h.ShortSeparator = "   "
	h.FullSeparator = "     "
	h.Ellipsis = "…"
	h.Styles = help.Styles{
		Ellipsis:       descStyle,
		ShortKey:       keyStyle,
		ShortDesc:      descStyle,
		ShortSeparator: descStyle,
		FullKey:        keyStyle,
		FullDesc:       descStyle,
		FullSeparator:  descStyle,
	}
	return h
}

func (m *Model) Init() tea.Cmd { return m.enter(m.screen) }

func (m *Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width, m.height = msg.Width, msg.Height
		m.resize()
		return m, nil
	case tea.KeyMsg:
		if key.Matches(msg, keyForceQuit) {
			return m, tea.Quit
		}
		if !m.screen.typing() && key.Matches(msg, keyHelpMore) {
			m.help.ShowAll = !m.help.ShowAll
			m.resize()
			return m, nil
		}
	case projectInfoMsg:
		m.info[msg.name] = msg.info
	}
	next, cmd := m.screen.update(msg)
	if next != m.screen {
		m.screen = next
		cmd = tea.Batch(cmd, m.enter(next))
	}
	return m, cmd
}

// enter makes s the current screen's size and starts it.
func (m *Model) enter(s screen) tea.Cmd {
	m.help.ShowAll = false
	w, h := m.contentSize()
	s.setSize(w, h)
	return s.init()
}

// resize tells the current screen how much room it has.
func (m *Model) resize() {
	w, h := m.contentSize()
	m.screen.setSize(w, h)
}

// frameSize returns the outer size of the frame: the terminal size, capped.
func (m *Model) frameSize() (int, int) {
	w, h := m.width, m.height
	if w == 0 || h == 0 {
		w, h = 80, 24
	}
	return min(w, maxFrameWidth), min(h, maxFrameHeight)
}

// innerWidth is the frame width minus its border and side padding.
func (m *Model) innerWidth() int {
	w, _ := m.frameSize()
	return max(w-2-4, 20)
}

// contentSize is the room a screen has between the header and the footer.
func (m *Model) contentSize() (int, int) {
	_, h := m.frameSize()
	inner := m.innerWidth()
	m.help.Width = inner
	footer := lipgloss.Height(m.help.View(m.keyMap()))
	// Border (2), header (1), gap under the header (1), rule (1), footer.
	return inner, max(h-2-1-1-1-footer, 3)
}

// keyMap adds the help toggle to the current screen's bindings.
func (m *Model) keyMap() keyMap {
	k := m.screen.keys()
	if m.screen.typing() {
		return k
	}
	toggle := keyHelpMore
	if m.help.ShowAll {
		toggle = keyHelpLess
	}
	return keyMap{
		short: append(append([]key.Binding{}, k.short...), toggle),
		full:  append(k.FullHelp(), []key.Binding{toggle, keyForceQuit}),
	}
}

func (m *Model) View() string {
	frameW, frameH := m.frameSize()
	inner := m.innerWidth()
	_, contentH := m.contentSize()

	content := lipgloss.NewStyle().
		Width(inner).
		Height(contentH).
		MaxHeight(contentH).
		Render(m.screen.view())
	body := lipgloss.JoinVertical(lipgloss.Left,
		m.header(inner),
		"",
		content,
		rule(inner),
		m.help.View(m.keyMap()),
	)
	frame := frameStyle.Width(frameW - 2).Height(frameH - 2).Render(body)
	if m.width > frameW || m.height > frameH {
		return lipgloss.Place(m.width, m.height, lipgloss.Center, lipgloss.Center, frame)
	}
	return frame
}

// header renders "Esque" on the left and the breadcrumb on the right. When the
// breadcrumb does not fit, the leading crumbs give way to "…".
func (m *Model) header(width int) string {
	brand := brandStyle.Render("Esque")
	crumbs := m.screen.crumbs()
	render := func(parts []string, elided bool) string {
		out := make([]string, 0, len(parts)+1)
		if elided {
			out = append(out, crumbStyle.Render("…"))
		}
		for i, p := range parts {
			if i == len(parts)-1 {
				out = append(out, crumbHere.Render(p))
			} else {
				out = append(out, crumbStyle.Render(p))
			}
		}
		return strings.Join(out, crumbStyle.Render(" › "))
	}
	trail := render(crumbs, false)
	for i := 1; lipgloss.Width(brand)+2+lipgloss.Width(trail) > width && i < len(crumbs); i++ {
		trail = render(crumbs[i:], true)
	}
	trail = truncate(trail, width-lipgloss.Width(brand)-2)
	gap := max(width-lipgloss.Width(brand)-lipgloss.Width(trail), 2)
	return brand + strings.Repeat(" ", gap) + trail
}

// now returns the current local time, or the fixed clock the tests pass.
func (m *Model) now() time.Time {
	if m.deps.Now != nil {
		return m.deps.Now()
	}
	return time.Now()
}

// nextID returns a number no earlier call returned, to tie timer messages to
// the state that started them.
func (m *Model) nextID() int {
	m.seq++
	return m.seq
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

// projectInfo is what the projects screen shows on a project's second line.
type projectInfo struct {
	loading bool
	last    time.Time
	count   int
	err     error
}

// projectInfoMsg carries the result of reading one project's backup list.
type projectInfoMsg struct {
	name string
	info projectInfo
}

// infoTimeout bounds the background read of each project's backup list.
const infoTimeout = 15 * time.Second

// fetchInfo reads the backup list of every project with S3 that the model
// has no information about yet. It uses backup.List, so it only reads.
func (m *Model) fetchInfo() tea.Cmd {
	if m.deps.OpenStore == nil {
		return nil
	}
	var cmds []tea.Cmd
	for _, p := range m.cfg.Projects {
		if p.S3 == nil {
			continue
		}
		if _, known := m.info[p.Name]; known {
			continue
		}
		m.info[p.Name] = projectInfo{loading: true}
		setup, name, open := *p.S3, p.Name, m.deps.OpenStore
		cmds = append(cmds, func() tea.Msg {
			store, err := open(setup)
			if err != nil {
				return projectInfoMsg{name: name, info: projectInfo{err: err}}
			}
			ctx, cancel := context.WithTimeout(context.Background(), infoTimeout)
			defer cancel()
			objects, err := backup.List(ctx, store)
			if err != nil {
				return projectInfoMsg{name: name, info: projectInfo{err: err}}
			}
			info := projectInfo{count: len(objects)}
			if len(objects) > 0 {
				info.last = objects[0].Modified
			}
			return projectInfoMsg{name: name, info: info}
		})
	}
	return tea.Batch(cmds...)
}

// forget drops what the model knows about a project's backups, so the
// projects screen reads them again.
func (m *Model) forget(name string) {
	delete(m.info, name)
}

// flashKind picks the colour of a status message.
type flashKind int

const (
	flashOK flashKind = iota
	flashWarn
	flashErr
)

// flash is a status message that clears itself after flashDuration.
type flash struct {
	kind flashKind
	text string
	id   int
}

// flashExpiredMsg clears the flash with the same id.
type flashExpiredMsg struct{ id int }

const flashDuration = 3 * time.Second

// set shows text and returns the timer that clears it.
func (f *flash) set(m *Model, kind flashKind, text string) tea.Cmd {
	f.kind, f.text, f.id = kind, text, m.nextID()
	id := f.id
	return tea.Tick(flashDuration, func(time.Time) tea.Msg { return flashExpiredMsg{id: id} })
}

// expire clears the flash if msg belongs to it.
func (f *flash) expire(msg flashExpiredMsg) {
	if msg.id == f.id {
		f.text = ""
	}
}

func (f flash) view(width int) string {
	if f.text == "" {
		return ""
	}
	return statusText(f.kind, f.text, width)
}

// statusText renders a one-off status line in the colour for its kind.
func statusText(kind flashKind, text string, width int) string {
	switch kind {
	case flashOK:
		return okStyle.Render(wrap(text, width))
	case flashWarn:
		return warnStyle.Render(wrap(text, width))
	}
	return errorStyle.Render(wrap(text, width))
}

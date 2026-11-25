package tui

import (
	"fmt"
	"io"
	"time"

	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/list"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/dustin/go-humanize"
)

// projectItem is one row of the projects list.
type projectItem struct {
	index  int
	name   string
	detail string
	warn   bool
}

func (p projectItem) FilterValue() string { return p.name }

// projectDelegate draws a project as a name line and a muted detail line.
type projectDelegate struct{}

func (projectDelegate) Height() int                         { return 2 }
func (projectDelegate) Spacing() int                        { return 1 }
func (projectDelegate) Update(tea.Msg, *list.Model) tea.Cmd { return nil }

func (projectDelegate) Render(w io.Writer, m list.Model, index int, item list.Item) {
	p, ok := item.(projectItem)
	if !ok {
		return
	}
	selected := index == m.Index()
	name := p.name
	if selected {
		name = selectedStyle.Render(name)
	}
	detail := mutedStyle.Render(p.detail)
	if p.warn {
		detail = warnStyle.Render(p.detail)
	}
	width := m.Width()
	fmt.Fprintf(w, "%s\n%s",
		truncate(marker(selected)+name, width),
		truncate("  "+detail, width))
}

// projects lists the configured projects.
type projects struct {
	m      *Model
	list   list.Model
	width  int
	height int
}

func newProjects(m *Model) *projects {
	l := list.New(nil, projectDelegate{}, 0, 0)
	l.SetShowTitle(false)
	l.SetShowStatusBar(false)
	l.SetShowHelp(false)
	l.SetFilteringEnabled(true)
	// The filter line shows only while typing a filter; see update.
	l.SetShowFilter(false)
	l.DisableQuitKeybindings()
	l.KeyMap.ShowFullHelp.SetEnabled(false)
	l.KeyMap.CloseFullHelp.SetEnabled(false)
	l.KeyMap.CursorUp = keyUp
	l.KeyMap.CursorDown = keyDown
	l.KeyMap.Filter = projectsKeys.Filter
	l.KeyMap.AcceptWhileFiltering = projectsKeys.Apply
	l.KeyMap.CancelWhileFiltering = projectsKeys.CancelFilter
	l.KeyMap.ClearFilter = projectsKeys.ClearFilter
	l.KeyMap.PrevPage = projectsKeys.PageUp
	l.KeyMap.NextPage = projectsKeys.PageDown
	l.Styles.TitleBar = lipgloss.NewStyle()
	l.FilterInput.Prompt = "Filter: "
	l.FilterInput.PromptStyle = accentStyle
	l.FilterInput.Cursor.Style = accentStyle
	l.Styles.NoItems = mutedStyle
	return &projects{m: m, list: l}
}

func (s *projects) init() tea.Cmd {
	cmd := s.m.fetchInfo()
	s.refresh()
	return cmd
}

func (s *projects) setSize(width, height int) {
	s.width, s.height = width, height
	s.sizeList()
}

// sizeList fits the list under the filter note when a filter is applied.
func (s *projects) sizeList() {
	h := s.height
	if s.list.FilterState() == list.FilterApplied {
		h--
	}
	s.list.SetSize(s.width, h)
}

func (s *projects) crumbs() []string { return []string{"Projects"} }

func (s *projects) typing() bool { return s.list.FilterState() == list.Filtering }

func (s *projects) keys() keyMap {
	k := projectsKeys
	switch s.list.FilterState() {
	case list.Filtering:
		return keyMap{short: []key.Binding{k.Apply, k.CancelFilter}}
	case list.FilterApplied:
		return keyMap{
			short: []key.Binding{k.Open, k.ClearFilter, k.Add, k.Quit},
			full: [][]key.Binding{
				{keyMove, k.Open, k.Add},
				{k.Filter, k.ClearFilter, k.Quit},
			},
		}
	}
	if len(s.m.cfg.Projects) == 0 {
		return keyMap{short: []key.Binding{k.Add, k.Quit}}
	}
	return keyMap{
		short: []key.Binding{k.Add, k.Open, k.Filter, k.Quit},
		full: [][]key.Binding{
			{keyMove, k.PageUp, k.PageDown},
			{k.Open, k.Add, k.Filter, k.Quit},
		},
	}
}

// refresh rebuilds the rows from the config and what the model knows about
// each project's backups. It keeps the cursor on the same project.
func (s *projects) refresh() {
	items := make([]list.Item, len(s.m.cfg.Projects))
	for i, p := range s.m.cfg.Projects {
		detail, warn := s.detail(i)
		items[i] = projectItem{index: i, name: p.Name, detail: detail, warn: warn}
	}
	cursor := s.list.Index()
	s.list.SetItems(items)
	if cursor < len(items) {
		s.list.Select(cursor)
	}
}

// detail returns the second line for project i and whether it is a warning.
func (s *projects) detail(i int) (string, bool) {
	p := s.m.cfg.Projects[i]
	if p.S3 == nil {
		return "never backed up · " + warnGlyph + " S3 not set up", true
	}
	info, known := s.m.info[p.Name]
	switch {
	case !known || info.loading:
		return "checking…", false
	case info.err != nil:
		return warnGlyph + " " + describe(info.err, p.S3.Endpoint), true
	case info.count == 0:
		return "never backed up · S3 ready", false
	}
	return "last backup " + ago(info.last, s.m.now()) + " · S3 ready", false
}

// ago says how long before now t was, such as "2 hours ago". A time ahead of
// the local clock counts as now.
func ago(t, now time.Time) string {
	if t.After(now) {
		t = now
	}
	return humanize.RelTime(t, now, "ago", "from now")
}

func (s *projects) update(msg tea.Msg) (screen, tea.Cmd) {
	switch msg := msg.(type) {
	case projectInfoMsg:
		s.refresh()
		return s, nil
	case tea.KeyMsg:
		if s.list.FilterState() == list.Filtering {
			break
		}
		switch {
		case key.Matches(msg, projectsKeys.Quit):
			return s, tea.Quit
		case key.Matches(msg, projectsKeys.Add):
			return newAddProject(s.m), nil
		case key.Matches(msg, projectsKeys.Open):
			if p, ok := s.list.SelectedItem().(projectItem); ok {
				return newProjectView(s.m, p.index), nil
			}
			return s, nil
		}
	}
	var cmd tea.Cmd
	s.list, cmd = s.list.Update(msg)
	s.list.SetShowFilter(s.list.FilterState() == list.Filtering)
	s.sizeList()
	return s, cmd
}

func (s *projects) view() string {
	if len(s.m.cfg.Projects) == 0 {
		msg := mutedStyle.Render("No projects yet. Press ") +
			keyStyle.Render("[a]") +
			mutedStyle.Render(" to add one.")
		return lipgloss.Place(s.width, s.height, lipgloss.Center, lipgloss.Center, msg)
	}
	if s.list.FilterState() == list.FilterApplied {
		note := mutedStyle.Render(truncate(fmt.Sprintf("Filter: %s", s.list.FilterValue()), s.width))
		return note + "\n" + s.list.View()
	}
	return s.list.View()
}

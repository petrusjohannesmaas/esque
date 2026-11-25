package tui

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/bubbles/table"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/dustin/go-humanize"

	"github.com/petrusjohannesmaas/esque/internal/backup"
	"github.com/petrusjohannesmaas/esque/internal/remote"
)

const (
	listLoading = iota
	listBrowsing
	listFolder
	listConfirm
	listRestoring
	listRestored
)

// Column widths of the backups table. The name column takes what is left.
const (
	colMarkW = 1
	colDateW = 17
	colAgeW  = 8
	colSizeW = 8
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
	m         *Model
	index     int
	restoring bool // opened from the Restore action
	phase     int
	objects   []remote.Object
	table     table.Model
	spin      spinner.Model
	folder    fieldSet
	dialog    *confirm
	errText   string
	path      string
	width     int
	height    int
}

func newBackupList(m *Model, index int, restoring bool) *backupList {
	s := &backupList{m: m, index: index, restoring: restoring, phase: listLoading}
	s.spin = spinner.New(spinner.WithSpinner(spinner.Dot), spinner.WithStyle(accentStyle))
	km := table.DefaultKeyMap()
	km.LineUp = keyUp
	km.LineDown = keyDown
	s.table = table.New(table.WithFocused(true), table.WithKeyMap(km))
	s.table.SetStyles(table.Styles{
		Header: mutedStyle.Bold(true).PaddingRight(1).
			BorderStyle(lipgloss.NormalBorder()).BorderBottom(true).BorderForeground(colFaint),
		Cell:     lipgloss.NewStyle().PaddingRight(1),
		Selected: selectedStyle,
	})
	return s
}

func (s *backupList) init() tea.Cmd {
	store, err := s.m.deps.OpenStore(*s.m.cfg.Projects[s.index].S3)
	if err != nil {
		s.phase = listBrowsing
		s.errText = describe(err, s.m.endpoint(s.index))
		return nil
	}
	ctx := context.Background()
	return tea.Batch(s.spin.Tick, func() tea.Msg {
		objects, err := backup.List(ctx, store)
		return listDoneMsg{objects: objects, err: err}
	})
}

func (s *backupList) setSize(width, height int) {
	s.width, s.height = width, height
	s.folder.setWidth(width)
	nameW := max(width-colMarkW-colDateW-colAgeW-colSizeW-5, 10)
	s.table.SetColumns([]table.Column{
		{Title: "", Width: colMarkW},
		{Title: "Backup", Width: nameW},
		{Title: "Date", Width: colDateW},
		{Title: "Age", Width: colAgeW},
		{Title: "Size", Width: colSizeW},
	})
	s.table.SetWidth(width)
	s.table.SetHeight(max(height-2, 3))
	s.fillRows()
}

func (s *backupList) crumbs() []string {
	last := "Backups"
	if s.restoring {
		last = "Restore"
	}
	return []string{"Projects", s.m.cfg.Projects[s.index].Name, last}
}

func (s *backupList) typing() bool { return s.phase == listFolder }

func (s *backupList) keys() keyMap {
	switch s.phase {
	case listBrowsing:
		if len(s.objects) == 0 {
			return keyMap{short: []key.Binding{keyBack}}
		}
		return keyMap{short: []key.Binding{keyMove, backupKeys.Restore, keyBack}}
	case listFolder:
		return keyMap{short: []key.Binding{backupKeys.Continue, keyBack}}
	case listConfirm:
		return s.dialog.keys()
	case listRestored:
		return keyMap{short: []key.Binding{keyBack}}
	}
	return keyMap{short: []key.Binding{keyForceQuit}}
}

// selectedName is the backup name, without its suffix, under the cursor.
func (s *backupList) selectedName() string {
	c := s.table.Cursor()
	if c < 0 || c >= len(s.objects) {
		return ""
	}
	return strings.TrimSuffix(s.objects[c].Name, ".sqlite.enc")
}

// fillRows rebuilds the table rows, with "▸" on the cursor row so the
// selection shows without colour too.
func (s *backupList) fillRows() {
	now := s.m.now()
	rows := make([]table.Row, len(s.objects))
	cursor := s.table.Cursor()
	for i, obj := range s.objects {
		mark := " "
		if i == cursor {
			mark = "▸"
		}
		rows[i] = table.Row{
			mark,
			strings.TrimSuffix(obj.Name, ".sqlite.enc"),
			obj.Modified.Local().Format("2 Jan 2006 15:04"),
			shortAge(obj.Modified, now),
			humanize.Bytes(uint64(obj.Size)),
		}
	}
	s.table.SetRows(rows)
}

func (s *backupList) update(msg tea.Msg) (screen, tea.Cmd) {
	switch msg := msg.(type) {
	case spinner.TickMsg:
		if s.phase != listLoading && s.phase != listRestoring {
			return s, nil
		}
		var cmd tea.Cmd
		s.spin, cmd = s.spin.Update(msg)
		return s, cmd
	case listDoneMsg:
		s.phase = listBrowsing
		if msg.err != nil {
			s.errText = describe(msg.err, s.m.endpoint(s.index))
			return s, nil
		}
		s.objects = msg.objects
		s.fillRows()
		s.table.SetCursor(0)
		s.fillRows()
		return s, nil
	case restoreDoneMsg:
		if msg.err != nil {
			s.phase = listFolder
			s.folder.fields[0].err = describe(msg.err, s.m.endpoint(s.index))
			return s, s.folder.focusOn(0)
		}
		s.path = msg.path
		s.phase = listRestored
		return s, nil
	case tea.KeyMsg:
		return s.updateKey(msg)
	}
	if s.phase == listFolder {
		return s, s.folder.updateFocused(msg)
	}
	return s, nil
}

func (s *backupList) updateKey(k tea.KeyMsg) (screen, tea.Cmd) {
	switch s.phase {
	case listFolder:
		return s.updateFolder(k)
	case listConfirm:
		done, yes := s.dialog.answer(k)
		if !done {
			return s, nil
		}
		s.dialog = nil
		if !yes {
			s.phase = listFolder
			return s, s.folder.focusOn(0)
		}
		return s, s.restore()
	case listRestored:
		if key.Matches(k, keyBack) {
			return newProjectView(s.m, s.index), nil
		}
		return s, nil
	case listLoading, listRestoring:
		return s, nil
	}
	switch {
	case key.Matches(k, keyBack):
		return newProjectView(s.m, s.index), nil
	case key.Matches(k, backupKeys.Restore):
		if len(s.objects) == 0 {
			return s, nil
		}
		return s, s.startFolder()
	}
	var cmd tea.Cmd
	s.table, cmd = s.table.Update(k)
	s.fillRows()
	return s, cmd
}

func (s *backupList) startFolder() tea.Cmd {
	f := newField("Restore into folder", "folder", "Existing files are not overwritten.")
	f.input.SetValue(filepath.Join(s.m.cfg.Projects[s.index].Dir, "restored"))
	s.folder = fieldSet{fields: []field{f}}
	s.folder.setWidth(s.width)
	s.phase = listFolder
	return s.folder.focusOn(0)
}

func (s *backupList) updateFolder(k tea.KeyMsg) (screen, tea.Cmd) {
	switch {
	case key.Matches(k, keyBack):
		s.phase = listBrowsing
		return s, nil
	case key.Matches(k, backupKeys.Continue):
		name := s.selectedName()
		folder := s.folder.fields[0].value()
		c := newConfirm(
			"Restore "+name+"?",
			"Into "+folder+"\n\nThe live database is not changed.",
			"restore", false, true)
		s.dialog = &c
		s.phase = listConfirm
		s.folder.fields[0].blur()
		return s, nil
	}
	return s, s.folder.updateFocused(k)
}

// restore runs the restore of the selected backup into the chosen folder.
func (s *backupList) restore() tea.Cmd {
	name := s.selectedName()
	dest := s.folder.fields[0].value()
	project := s.m.cfg.Projects[s.index]
	s.folder.clearErrors()
	store, err := s.m.deps.OpenStore(*project.S3)
	if err != nil {
		s.phase = listFolder
		s.folder.fields[0].err = describe(err, s.m.endpoint(s.index))
		return s.folder.focusOn(0)
	}
	s.phase = listRestoring
	ctx := context.Background()
	return tea.Batch(s.spin.Tick, func() tea.Msg {
		path, err := backup.Restore(ctx, project, name, dest, store)
		return restoreDoneMsg{path: path, err: err}
	})
}

func (s *backupList) view() string {
	switch s.phase {
	case listLoading:
		return s.spin.View() + " " + wrap("Listing backups…", s.width-2)
	case listRestoring:
		return s.spin.View() + " " + wrap("Restoring "+s.selectedName()+"…", s.width-2)
	case listFolder:
		return mutedStyle.Render(truncate("Backup "+s.selectedName(), s.width)) + "\n\n" + s.folder.view(s.height-2)
	case listConfirm:
		return s.dialog.view(s.width, s.height)
	case listRestored:
		return okStyle.Render(wrap(okGlyph+" Restored to "+s.path, s.width))
	}
	if s.errText != "" {
		return errorStyle.Render(wrap(s.errText, s.width))
	}
	if len(s.objects) == 0 {
		return mutedStyle.Render("No backups yet.")
	}
	return s.table.View()
}

// shortAge says how long ago t was in a few characters, such as "3d ago".
func shortAge(t, now time.Time) string {
	d := now.Sub(t)
	switch {
	case d < time.Minute:
		return "now"
	case d < time.Hour:
		return fmt.Sprintf("%dm ago", int(d.Minutes()))
	case d < 24*time.Hour:
		return fmt.Sprintf("%dh ago", int(d.Hours()))
	case d < 60*24*time.Hour:
		return fmt.Sprintf("%dd ago", int(d.Hours()/24))
	case d < 365*24*time.Hour:
		return fmt.Sprintf("%dmo ago", int(d.Hours()/24/30))
	}
	return fmt.Sprintf("%dy ago", int(d.Hours()/24/365))
}

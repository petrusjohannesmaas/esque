package tui

import (
	"strings"

	"github.com/charmbracelet/bubbles/key"
	tea "github.com/charmbracelet/bubbletea"

	"github.com/petrusjohannesmaas/esque/internal/config"
	"github.com/petrusjohannesmaas/esque/internal/sqlitefile"
)

const (
	fieldName = iota
	fieldDir
	fieldPassphrase
)

type addProject struct {
	m      *Model
	form   fieldSet
	status string
	height int
}

func newAddProject(m *Model) *addProject {
	fields := []field{
		newField("Name", "blog", ""),
		newField("Directory", "/srv/blog", "Folder with the SQLite file."),
		newField("Passphrase", "encrypts every backup",
			"Keep a copy. Backups cannot be restored without it."),
	}
	fields[fieldPassphrase].secret()
	return &addProject{m: m, form: fieldSet{fields: fields}}
}

func (s *addProject) init() tea.Cmd { return s.form.focusOn(s.form.focus) }

func (s *addProject) setSize(width, height int) {
	s.form.setWidth(width)
	s.height = height
}

func (s *addProject) crumbs() []string { return []string{"Projects", "Add project"} }

func (s *addProject) typing() bool { return true }

func (s *addProject) keys() keyMap {
	return keyMap{short: []key.Binding{formKeys.Next, formKeys.Prev, formKeys.Save, keyCancel}}
}

func (s *addProject) update(msg tea.Msg) (screen, tea.Cmd) {
	if k, ok := msg.(tea.KeyMsg); ok {
		switch {
		case key.Matches(k, keyCancel):
			return newProjects(s.m), nil
		case key.Matches(k, formKeys.Next):
			return s, s.form.next()
		case key.Matches(k, formKeys.Prev):
			return s, s.form.prev()
		case key.Matches(k, formKeys.Save):
			return s.submit()
		}
	}
	return s, s.form.updateFocused(msg)
}

func (s *addProject) submit() (screen, tea.Cmd) {
	f := s.form.fields
	name := f[fieldName].value()
	dir := f[fieldDir].value()
	passphrase := f[fieldPassphrase].input.Value()
	s.status = ""
	s.form.clearErrors()

	files, err := sqlitefile.Find(dir)
	if err != nil {
		f[fieldDir].err = err.Error()
		_, cmd := s.form.focusFirstError()
		return s, cmd
	}
	if len(files) == 0 {
		f[fieldDir].err = "No SQLite database found in " + dir
		_, cmd := s.form.focusFirstError()
		return s, cmd
	}
	p := config.Project{Name: name, Dir: dir, Passphrase: passphrase}
	if len(files) > 1 {
		return newPicker(s, p, files), nil
	}
	p.DB = files[0]
	if err := s.m.addProject(p); err != nil {
		s.status = err.Error()
		return s, nil
	}
	return newProjects(s.m), nil
}

func (s *addProject) view() string {
	room := s.height
	var status string
	if s.status != "" {
		status = statusText(flashErr, s.status, s.form.width)
		room -= strings.Count(status, "\n") + 2
	}
	out := s.form.view(room)
	if status != "" {
		out += "\n\n" + status
	}
	return out
}

// picker asks which SQLite file in the directory the project uses.
type picker struct {
	form    *addProject
	project config.Project
	files   []string
	cursor  int
	width   int
}

func newPicker(form *addProject, project config.Project, files []string) *picker {
	return &picker{form: form, project: project, files: files}
}

func (s *picker) init() tea.Cmd { return nil }

func (s *picker) setSize(width, _ int) { s.width = width }

func (s *picker) crumbs() []string { return []string{"Projects", "Add project", "Database"} }

func (s *picker) typing() bool { return false }

func (s *picker) keys() keyMap {
	return keyMap{short: []key.Binding{keyMove, pickerKeys.Choose, keyBack}}
}

func (s *picker) update(msg tea.Msg) (screen, tea.Cmd) {
	k, ok := msg.(tea.KeyMsg)
	if !ok {
		return s, nil
	}
	switch {
	case key.Matches(k, keyBack):
		return s.form, nil
	case key.Matches(k, keyUp):
		if s.cursor > 0 {
			s.cursor--
		}
	case key.Matches(k, keyDown):
		if s.cursor < len(s.files)-1 {
			s.cursor++
		}
	case key.Matches(k, pickerKeys.Choose):
		s.project.DB = s.files[s.cursor]
		if err := s.form.m.addProject(s.project); err != nil {
			s.form.status = err.Error()
			return s.form, nil
		}
		return newProjects(s.form.m), nil
	}
	return s, nil
}

func (s *picker) view() string {
	var b strings.Builder
	b.WriteString(titleStyle.Render("Choose a database") + "\n")
	b.WriteString(mutedStyle.Render(truncate("in "+s.project.Dir, s.width)) + "\n\n")
	for i, file := range s.files {
		name := file
		if i == s.cursor {
			name = selectedStyle.Render(file)
		}
		b.WriteString(truncate(marker(i == s.cursor)+name, s.width) + "\n")
	}
	return b.String()
}

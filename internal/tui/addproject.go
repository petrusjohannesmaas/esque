package tui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"

	"github.com/petrusjohannesmaas/esque/internal/config"
	"github.com/petrusjohannesmaas/esque/internal/sqlitefile"
)

const (
	fieldName = iota
	fieldDir
	fieldPassphrase
	fieldCount
)

type addProject struct {
	m      *Model
	inputs [fieldCount]textinput.Model
	focus  int
	status string
}

func newAddProject(m *Model) *addProject {
	s := &addProject{m: m}
	for i := range s.inputs {
		s.inputs[i] = textinput.New()
		s.inputs[i].Width = 60
	}
	s.inputs[fieldName].Placeholder = "blog"
	s.inputs[fieldDir].Placeholder = "/srv/blog"
	s.inputs[fieldPassphrase].EchoMode = textinput.EchoPassword
	s.inputs[fieldName].Focus()
	return s
}

func (s *addProject) update(msg tea.Msg) (screen, tea.Cmd) {
	key, ok := msg.(tea.KeyMsg)
	if ok {
		switch key.String() {
		case "esc":
			return newMenu(s.m), nil
		case "tab":
			return s, s.focusField((s.focus + 1) % fieldCount)
		case "shift+tab":
			return s, s.focusField((s.focus + fieldCount - 1) % fieldCount)
		case "enter":
			return s.submit()
		}
	}
	var cmd tea.Cmd
	s.inputs[s.focus], cmd = s.inputs[s.focus].Update(msg)
	return s, cmd
}

func (s *addProject) focusField(i int) tea.Cmd {
	s.focus = i
	for j := range s.inputs {
		if j == i {
			s.inputs[j].Focus()
		} else {
			s.inputs[j].Blur()
		}
	}
	return textinput.Blink
}

func (s *addProject) submit() (screen, tea.Cmd) {
	name := strings.TrimSpace(s.inputs[fieldName].Value())
	dir := strings.TrimSpace(s.inputs[fieldDir].Value())
	passphrase := s.inputs[fieldPassphrase].Value()

	files, err := sqlitefile.Find(dir)
	if err != nil {
		s.status = err.Error()
		return s, nil
	}
	if len(files) == 0 {
		s.status = "No SQLite database found in " + dir
		return s, nil
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
	return newMenu(s.m), nil
}

func (s *addProject) view() string {
	var b strings.Builder
	b.WriteString("Add project\n\n")
	labels := [fieldCount]string{"Name", "Directory", "Passphrase"}
	for i := range s.inputs {
		fmt.Fprintf(&b, "%s\n%s\n", labels[i], s.inputs[i].View())
	}
	if s.status != "" {
		fmt.Fprintf(&b, "\n%s\n", s.status)
	}
	b.WriteString("\ntab next • enter save • esc cancel\n")
	return b.String()
}

// picker asks which SQLite file in the directory the project uses.
type picker struct {
	form    *addProject
	project config.Project
	files   []string
	cursor  int
}

func newPicker(form *addProject, project config.Project, files []string) *picker {
	return &picker{form: form, project: project, files: files}
}

func (s *picker) update(msg tea.Msg) (screen, tea.Cmd) {
	key, ok := msg.(tea.KeyMsg)
	if !ok {
		return s, nil
	}
	switch key.String() {
	case "esc":
		return s.form, nil
	case "up", "k":
		if s.cursor > 0 {
			s.cursor--
		}
	case "down", "j":
		if s.cursor < len(s.files)-1 {
			s.cursor++
		}
	case "enter":
		s.project.DB = s.files[s.cursor]
		if err := s.form.m.addProject(s.project); err != nil {
			s.form.status = err.Error()
			return s.form, nil
		}
		return newMenu(s.form.m), nil
	}
	return s, nil
}

func (s *picker) view() string {
	var b strings.Builder
	b.WriteString("Choose the SQLite database file\n\n")
	for i, file := range s.files {
		fmt.Fprintf(&b, "%s %s\n", cursorMark(i == s.cursor), file)
	}
	b.WriteString("\nup/down move • enter choose • esc back\n")
	return b.String()
}

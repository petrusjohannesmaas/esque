package tui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"

	"github.com/petrusjohannesmaas/esque/internal/config"
)

const (
	s3Endpoint = iota
	s3Region
	s3Bucket
	s3Prefix
	s3AccessKey
	s3SecretKey
	s3FieldCount
)

var s3FieldNames = [s3FieldCount]string{
	"Endpoint",
	"Region",
	"Bucket",
	"Prefix",
	"Access key ID",
	"Secret key",
}

// s3Form creates or edits the selected project's S3 setup.
type s3Form struct {
	m      *Model
	index  int
	inputs [s3FieldCount]textinput.Model
	focus  int
	status string
}

func newS3Form(m *Model, index int) *s3Form {
	s := &s3Form{m: m, index: index}
	for i := range s.inputs {
		s.inputs[i] = textinput.New()
		s.inputs[i].Width = 60
	}
	s.inputs[s3SecretKey].EchoMode = textinput.EchoPassword
	s.inputs[s3SecretKey].Placeholder = "leave empty to keep the current key"
	if existing := m.cfg.Projects[index].S3; existing != nil {
		s.inputs[s3Endpoint].SetValue(existing.Endpoint)
		s.inputs[s3Region].SetValue(existing.Region)
		s.inputs[s3Bucket].SetValue(existing.Bucket)
		s.inputs[s3Prefix].SetValue(existing.Prefix)
		s.inputs[s3AccessKey].SetValue(existing.AccessKeyID)
	}
	s.inputs[s3Endpoint].Focus()
	return s
}

func (s *s3Form) update(msg tea.Msg) (screen, tea.Cmd) {
	key, ok := msg.(tea.KeyMsg)
	if ok {
		switch key.String() {
		case "esc":
			return newProjectView(s.m, s.index), nil
		case "tab":
			return s, s.focusField((s.focus + 1) % s3FieldCount)
		case "shift+tab":
			return s, s.focusField((s.focus + s3FieldCount - 1) % s3FieldCount)
		case "enter":
			return s.submit()
		}
	}
	var cmd tea.Cmd
	s.inputs[s.focus], cmd = s.inputs[s.focus].Update(msg)
	return s, cmd
}

func (s *s3Form) focusField(i int) tea.Cmd {
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

func (s *s3Form) submit() (screen, tea.Cmd) {
	setup := config.S3Setup{
		Endpoint:    strings.TrimSpace(s.inputs[s3Endpoint].Value()),
		Region:      strings.TrimSpace(s.inputs[s3Region].Value()),
		Bucket:      strings.TrimSpace(s.inputs[s3Bucket].Value()),
		Prefix:      strings.TrimSpace(s.inputs[s3Prefix].Value()),
		AccessKeyID: strings.TrimSpace(s.inputs[s3AccessKey].Value()),
		SecretKey:   s.inputs[s3SecretKey].Value(),
	}
	if err := s.m.cfg.SetS3(s.m.cfg.Projects[s.index].Name, setup); err != nil {
		s.status = err.Error()
		return s, nil
	}
	if err := config.Save(s.m.cfgPath, s.m.cfg); err != nil {
		s.status = err.Error()
		return s, nil
	}
	return newProjectView(s.m, s.index), nil
}

func (s *s3Form) view() string {
	var b strings.Builder
	fmt.Fprintf(&b, "S3 setup for %s\n\n", s.m.cfg.Projects[s.index].Name)
	for i := range s.inputs {
		fmt.Fprintf(&b, "%s\n%s\n", s3FieldNames[i], s.inputs[i].View())
	}
	if s.status != "" {
		fmt.Fprintf(&b, "\n%s\n", s.status)
	}
	b.WriteString("\ntab next • enter save • esc cancel\n")
	return b.String()
}

package tui

import (
	"context"
	"fmt"
	"net"
	"net/url"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/spinner"
	tea "github.com/charmbracelet/bubbletea"

	"github.com/petrusjohannesmaas/esque/internal/backup"
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

// s3TestTimeout bounds the connection test.
const s3TestTimeout = 15 * time.Second

// s3TestMsg carries the result of a connection test.
type s3TestMsg struct {
	id    int
	count int
	err   error
}

// s3Form creates or edits the selected project's S3 setup.
type s3Form struct {
	m       *Model
	index   int
	form    fieldSet
	spin    spinner.Model
	testing bool
	testID  int
	status  string
	kind    flashKind
	height  int
}

func newS3Form(m *Model, index int) *s3Form {
	p := m.cfg.Projects[index]
	existing := p.S3
	secretHint := ""
	secretPlaceholder := "secret access key"
	if existing != nil {
		secretPlaceholder = "unchanged"
		secretHint = "Leave empty to keep the current key."
	}
	fields := []field{
		newField("Endpoint", "http://localhost:3900", ""),
		newField("Region", "garage", ""),
		newField("Bucket", "backups", "Must already exist."),
		newField("Prefix", p.Name+"/", "Optional."),
		newField("Access key ID", "GK31c2f218a2e44f485b94239e", ""),
		newField("Secret key", secretPlaceholder, secretHint),
	}
	fields[s3SecretKey].secret()
	if existing != nil {
		fields[s3Endpoint].input.SetValue(existing.Endpoint)
		fields[s3Region].input.SetValue(existing.Region)
		fields[s3Bucket].input.SetValue(existing.Bucket)
		fields[s3Prefix].input.SetValue(existing.Prefix)
		fields[s3AccessKey].input.SetValue(existing.AccessKeyID)
	}
	s := &s3Form{m: m, index: index, form: fieldSet{fields: fields}}
	s.spin = spinner.New(spinner.WithSpinner(spinner.Dot), spinner.WithStyle(accentStyle))
	return s
}

func (s *s3Form) init() tea.Cmd { return s.form.focusOn(s3Endpoint) }

func (s *s3Form) setSize(width, height int) {
	s.form.setWidth(width)
	s.height = height
}

func (s *s3Form) crumbs() []string {
	return []string{"Projects", s.m.cfg.Projects[s.index].Name, "S3 setup"}
}

func (s *s3Form) typing() bool { return true }

func (s *s3Form) keys() keyMap {
	short := []key.Binding{formKeys.Next, formKeys.Save}
	if s.m.deps.OpenStore != nil {
		short = append(short, formKeys.Test)
	}
	short = append(short, keyCancel, formKeys.Prev)
	return keyMap{short: short}
}

func (s *s3Form) update(msg tea.Msg) (screen, tea.Cmd) {
	switch msg := msg.(type) {
	case spinner.TickMsg:
		if !s.testing {
			return s, nil
		}
		var cmd tea.Cmd
		s.spin, cmd = s.spin.Update(msg)
		return s, cmd
	case s3TestMsg:
		if msg.id != s.testID {
			return s, nil
		}
		s.testing = false
		if msg.err != nil {
			s.kind, s.status = flashErr, describe(msg.err, s.setup().Endpoint)
		} else {
			s.kind, s.status = flashOK, fmt.Sprintf("%s Connected · %s", okGlyph, strings.ToLower(backupCount(msg.count)))
		}
		return s, nil
	case tea.KeyMsg:
		switch {
		case key.Matches(msg, keyCancel):
			return newProjectView(s.m, s.index), nil
		case key.Matches(msg, formKeys.Next):
			return s, s.form.next()
		case key.Matches(msg, formKeys.Prev):
			return s, s.form.prev()
		case key.Matches(msg, formKeys.Save):
			return s.submit()
		case key.Matches(msg, formKeys.Test):
			return s, s.test()
		}
	}
	return s, s.form.updateFocused(msg)
}

// setup reads the form. An empty secret key keeps the stored one.
func (s *s3Form) setup() config.S3Setup {
	f := s.form.fields
	setup := config.S3Setup{
		Endpoint:    f[s3Endpoint].value(),
		Region:      f[s3Region].value(),
		Bucket:      f[s3Bucket].value(),
		Prefix:      f[s3Prefix].value(),
		AccessKeyID: f[s3AccessKey].value(),
		SecretKey:   f[s3SecretKey].input.Value(),
	}
	return setup
}

// validate marks each field that fails and moves the focus to the first one.
// It reports whether the form is valid.
func (s *s3Form) validate() (bool, tea.Cmd) {
	s.form.clearErrors()
	f := s.form.fields
	f[s3Endpoint].err = checkEndpoint(f[s3Endpoint].value())
	f[s3Bucket].err = checkBucket(f[s3Bucket].value())
	failed, cmd := s.form.focusFirstError()
	return !failed, cmd
}

func (s *s3Form) submit() (screen, tea.Cmd) {
	s.status = ""
	if ok, cmd := s.validate(); !ok {
		return s, cmd
	}
	setup := s.setup()
	name := s.m.cfg.Projects[s.index].Name
	if err := s.m.change(func(cfg *config.Config) error { return cfg.SetS3(name, setup) }); err != nil {
		s.kind, s.status = flashErr, err.Error()
		return s, nil
	}
	s.m.forget(name)
	return newProjectView(s.m, s.index).withFlash(flashOK, okGlyph+" S3 settings saved"), nil
}

// test lists the bucket with the settings in the form, without saving them.
func (s *s3Form) test() tea.Cmd {
	if s.m.deps.OpenStore == nil {
		return nil
	}
	s.status = ""
	if ok, cmd := s.validate(); !ok {
		return cmd
	}
	setup := s.setup()
	if setup.SecretKey == "" {
		if existing := s.m.cfg.Projects[s.index].S3; existing != nil {
			setup.SecretKey = existing.SecretKey
		}
	}
	s.testing = true
	s.testID = s.m.nextID()
	id, open := s.testID, s.m.deps.OpenStore
	return tea.Batch(s.spin.Tick, func() tea.Msg {
		store, err := open(setup)
		if err != nil {
			return s3TestMsg{id: id, err: err}
		}
		ctx, cancel := context.WithTimeout(context.Background(), s3TestTimeout)
		defer cancel()
		objects, err := backup.List(ctx, store)
		return s3TestMsg{id: id, count: len(objects), err: err}
	})
}

func (s *s3Form) view() string {
	var footer string
	switch {
	case s.testing:
		footer = s.spin.View() + " " + mutedStyle.Render("Testing…")
	case s.status != "":
		footer = statusText(s.kind, s.status, s.form.width)
	}
	room := s.height
	if footer != "" {
		room -= strings.Count(footer, "\n") + 2
	}
	out := s.form.view(room)
	if footer != "" {
		out += "\n\n" + footer
	}
	return out
}

// backupCount says how many backups there are, in words.
func backupCount(n int) string {
	switch n {
	case 0:
		return "No backups yet"
	case 1:
		return "1 backup"
	}
	return fmt.Sprintf("%d backups", n)
}

// checkEndpoint returns why endpoint is not a usable S3 URL, or "".
func checkEndpoint(endpoint string) string {
	if endpoint == "" {
		return "Required."
	}
	u, err := url.Parse(endpoint)
	if err != nil || (u.Scheme != "http" && u.Scheme != "https") {
		return "Must start with http:// or https://."
	}
	if u.Host == "" {
		return "Missing host name."
	}
	if u.Path != "" && u.Path != "/" {
		return "Remove the path."
	}
	return ""
}

// checkBucket returns why name breaks the S3 bucket naming rules, or "".
func checkBucket(name string) string {
	switch {
	case name == "":
		return "Required."
	case len(name) < 3 || len(name) > 63:
		return "Use 3 to 63 characters."
	case strings.Contains(name, ".."):
		return "No two dots in a row."
	case net.ParseIP(name) != nil:
		return "Cannot be an IP address."
	}
	for _, r := range name {
		if !(r >= 'a' && r <= 'z' || r >= '0' && r <= '9' || r == '.' || r == '-') {
			return "Lowercase letters, digits, dots and hyphens only."
		}
	}
	if !isAlnum(name[0]) || !isAlnum(name[len(name)-1]) {
		return "Must start and end with a letter or digit."
	}
	return ""
}

func isAlnum(c byte) bool { return c >= 'a' && c <= 'z' || c >= '0' && c <= '9' }

package tui

import (
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
)

// field is one labelled text input with a hint and an error line.
type field struct {
	label string
	hint  string
	input textinput.Model
	err   string
}

func newField(label, placeholder, hint string) field {
	in := textinput.New()
	in.Prompt = "› "
	in.Placeholder = placeholder
	in.PlaceholderStyle = mutedStyle
	in.PromptStyle = mutedStyle
	in.Cursor.Style = accentStyle
	return field{label: label, hint: hint, input: in}
}

// secret makes the field show "•" for each character typed.
func (f *field) secret() {
	f.input.EchoMode = textinput.EchoPassword
	f.input.EchoCharacter = '•'
}

func (f *field) focus() tea.Cmd {
	f.input.PromptStyle = accentStyle
	return f.input.Focus()
}

func (f *field) blur() {
	f.input.PromptStyle = mutedStyle
	f.input.Blur()
}

// value is the trimmed text typed into the field. A placeholder is never a
// value.
func (f *field) value() string { return strings.TrimSpace(f.input.Value()) }

// fieldSet is the focus handling and layout shared by the forms.
type fieldSet struct {
	fields []field
	focus  int
	width  int
}

func (fs *fieldSet) setWidth(width int) {
	fs.width = width
	for i := range fs.fields {
		fs.fields[i].input.Width = max(width-lenPrompt-1, 10)
	}
}

const lenPrompt = 2 // the width of "› "

// focusOn moves the focus to field i.
func (fs *fieldSet) focusOn(i int) tea.Cmd {
	fs.focus = i
	var cmd tea.Cmd
	for j := range fs.fields {
		if j == i {
			cmd = fs.fields[j].focus()
		} else {
			fs.fields[j].blur()
		}
	}
	return cmd
}

func (fs *fieldSet) next() tea.Cmd { return fs.focusOn((fs.focus + 1) % len(fs.fields)) }

func (fs *fieldSet) prev() tea.Cmd {
	return fs.focusOn((fs.focus + len(fs.fields) - 1) % len(fs.fields))
}

// updateFocused passes msg to the focused input.
func (fs *fieldSet) updateFocused(msg tea.Msg) tea.Cmd {
	var cmd tea.Cmd
	fs.fields[fs.focus].input, cmd = fs.fields[fs.focus].input.Update(msg)
	return cmd
}

// clearErrors removes every field's error.
func (fs *fieldSet) clearErrors() {
	for i := range fs.fields {
		fs.fields[i].err = ""
	}
}

// focusFirstError moves the focus to the first field with an error and
// reports whether there was one.
func (fs *fieldSet) focusFirstError() (bool, tea.Cmd) {
	for i := range fs.fields {
		if fs.fields[i].err != "" {
			return true, fs.focusOn(i)
		}
	}
	return false, nil
}

// view renders the fields in height rows. It puts a blank line between
// fields when they fit with one.
func (fs *fieldSet) view(height int) string {
	blocks := make([]string, len(fs.fields))
	lines := 0
	for i := range fs.fields {
		blocks[i] = fs.fieldView(i)
		lines += strings.Count(blocks[i], "\n") + 1
	}
	sep := "\n"
	if lines+len(fs.fields)-1 <= height {
		sep = "\n\n"
	}
	return strings.Join(blocks, sep)
}

func (fs *fieldSet) fieldView(i int) string {
	f := fs.fields[i]
	focused := i == fs.focus
	label := mutedStyle.Render(f.label)
	if focused {
		label = titleStyle.Render(f.label)
	}
	out := []string{label, f.input.View()}
	switch {
	case f.err != "":
		out = append(out, errorStyle.Render(wrap(f.err, fs.width-lenPrompt)))
	case focused && f.hint != "":
		out = append(out, mutedStyle.Render(wrap(f.hint, fs.width-lenPrompt)))
	}
	for j := 2; j < len(out); j++ {
		out[j] = indent(out[j], lenPrompt)
	}
	return strings.Join(out, "\n")
}

// indent puts n spaces in front of every line of s.
func indent(s string, n int) string {
	pad := strings.Repeat(" ", n)
	return pad + strings.ReplaceAll(s, "\n", "\n"+pad)
}

package tui

import (
	"github.com/charmbracelet/bubbles/key"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// confirm is a yes/no question drawn as a bordered box in the middle of the
// content area.
type confirm struct {
	title      string
	body       string
	yes, no    key.Binding
	enter      key.Binding
	defaultYes bool
	danger     bool
}

// newConfirm builds a dialog. yesDesc names the action, such as "remove".
// enter picks yes when defaultYes is set and cancel otherwise.
func newConfirm(title, body, yesDesc string, danger, defaultYes bool) confirm {
	c := confirm{title: title, body: body, danger: danger, defaultYes: defaultYes}
	c.yes = key.NewBinding(key.WithKeys(dialogKeys.Yes.Keys()...), key.WithHelp(dialogKeys.Yes.Help().Key, yesDesc))
	c.no = dialogKeys.No
	enterDesc := "cancel"
	if defaultYes {
		enterDesc = yesDesc
	}
	c.enter = key.NewBinding(key.WithKeys(dialogKeys.Enter.Keys()...), key.WithHelp(dialogKeys.Enter.Help().Key, enterDesc))
	return c
}

// answer reports whether msg settles the dialog, and if so whether the answer
// is yes.
func (c confirm) answer(msg tea.KeyMsg) (done, yes bool) {
	switch {
	case key.Matches(msg, c.yes):
		return true, true
	case key.Matches(msg, c.no):
		return true, false
	case key.Matches(msg, c.enter):
		return true, c.defaultYes
	}
	return false, false
}

func (c confirm) keys() keyMap {
	return keyMap{short: []key.Binding{c.yes, c.no, c.enter}}
}

func (c confirm) view(width, height int) string {
	boxW := min(width-2, 60)
	textW := boxW - 2 - 6
	title := boldStyle.Render(c.title)
	if c.danger {
		title = dangerStyle.Bold(true).Render(c.title)
	}
	yesKey := keyStyle.Render(c.yes.Help().Key) + " " + descStyle.Render(c.yes.Help().Desc)
	if c.danger {
		yesKey = dangerStyle.Render(c.yes.Help().Key) + " " + dangerStyle.Render(c.yes.Help().Desc)
	}
	noKey := keyStyle.Render(c.no.Help().Key) + " " + descStyle.Render(c.no.Help().Desc)
	choices := yesKey + "   " + noKey
	inner := lipgloss.JoinVertical(lipgloss.Left,
		wrap(title, textW),
		"",
		wrap(c.body, textW),
		"",
		choices,
	)
	style := dialogStyle.Width(boxW - 2)
	if c.danger {
		style = style.BorderForeground(colRed)
	}
	return lipgloss.Place(width, height, lipgloss.Center, lipgloss.Center, style.Render(inner))
}

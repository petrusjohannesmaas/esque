package tui

import "github.com/charmbracelet/bubbles/key"

// bind defines a key binding whose help shows as "[label] desc".
func bind(label, desc string, keys ...string) key.Binding {
	return key.NewBinding(key.WithKeys(keys...), key.WithHelp("["+label+"]", desc))
}

// keyMap is the help.KeyMap a screen hands to the footer.
type keyMap struct {
	short []key.Binding
	full  [][]key.Binding
}

func (k keyMap) ShortHelp() []key.Binding { return k.short }

func (k keyMap) FullHelp() [][]key.Binding {
	if k.full == nil {
		return [][]key.Binding{k.short}
	}
	return k.full
}

// Keys every screen shares.
var (
	keyUp        = bind("↑", "up", "up", "k")
	keyDown      = bind("↓", "down", "down", "j")
	keyMove      = bind("↑/↓", "move", "up", "down", "k", "j")
	keyBack      = bind("esc", "back", "esc")
	keyCancel    = bind("esc", "cancel", "esc")
	keyHelpMore  = bind("?", "more", "?")
	keyHelpLess  = bind("?", "less", "?")
	keyForceQuit = bind("ctrl+c", "quit", "ctrl+c")
)

// Projects screen.
var projectsKeys = struct {
	Add, Open, Filter, Quit, Apply, CancelFilter, ClearFilter, PageUp, PageDown key.Binding
}{
	Add:          bind("a", "add project", "a"),
	Open:         bind("enter", "open", "enter"),
	Filter:       bind("/", "filter", "/"),
	Quit:         bind("q", "quit", "q"),
	Apply:        bind("enter", "apply filter", "enter"),
	CancelFilter: bind("esc", "cancel", "esc"),
	ClearFilter:  bind("esc", "clear filter", "esc"),
	PageUp:       bind("pgup", "previous page", "pgup", "left", "h"),
	PageDown:     bind("pgdn", "next page", "pgdown", "right", "l"),
}

// Project menu.
var menuKeys = struct{ Choose key.Binding }{
	Choose: bind("enter", "choose", "enter"),
}

// Confirmation dialogs. The descriptions change per dialog, so these are
// templates that confirmKeys fills in.
var dialogKeys = struct{ Yes, No, Enter key.Binding }{
	Yes:   bind("y", "confirm", "y", "Y"),
	No:    bind("n", "cancel", "n", "N", "esc"),
	Enter: bind("enter", "default", "enter"),
}

// Forms: S3 setup, add project, rename.
var formKeys = struct{ Next, Prev, Save, Test key.Binding }{
	Next: bind("tab", "next", "tab", "down"),
	Prev: bind("shift+tab", "prev", "shift+tab", "up"),
	Save: bind("enter", "save", "enter"),
	Test: bind("ctrl+t", "test", "ctrl+t"),
}

// Backup and restore.
var backupKeys = struct{ Start, Restore, Continue key.Binding }{
	Start:    bind("enter", "back up", "enter"),
	Restore:  bind("r", "restore", "r", "enter"),
	Continue: bind("enter", "continue", "enter"),
}

// Picker for the database file.
var pickerKeys = struct{ Choose key.Binding }{
	Choose: bind("enter", "choose", "enter"),
}

// Reveal passphrase.
var revealKeys = struct{ Hide key.Binding }{
	// Any key hides the passphrase. The listed keys only give the binding
	// something to hold, since help hides a binding with no keys.
	Hide: bind("any key", "hide", "esc", "enter", " "),
}

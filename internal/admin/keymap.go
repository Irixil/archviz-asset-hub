package admin

import "github.com/charmbracelet/bubbles/key"

type KeyMap struct {
	Screen1    key.Binding
	Screen2    key.Binding
	Screen3    key.Binding
	Screen4    key.Binding
	Screen5    key.Binding
	Refresh    key.Binding
	RefreshAll key.Binding
	Quit       key.Binding
	Help       key.Binding
	Up         key.Binding
	Down       key.Binding
	GotoTop    key.Binding
	GotoBottom key.Binding
	Search     key.Binding
	Filter     key.Binding
	Tab        key.Binding
	Enter      key.Binding
	Escape     key.Binding
	NextPage   key.Binding
	PrevPage   key.Binding
	Space      key.Binding
}

var DefaultKeyMap = KeyMap{
	Screen1: key.NewBinding(
		key.WithKeys("1"),
		key.WithHelp("1", "overview"),
	),
	Screen2: key.NewBinding(
		key.WithKeys("2"),
		key.WithHelp("2", "users"),
	),
	Screen3: key.NewBinding(
		key.WithKeys("3"),
		key.WithHelp("3", "activity"),
	),
	Screen4: key.NewBinding(
		key.WithKeys("4"),
		key.WithHelp("4", "storage"),
	),
	Screen5: key.NewBinding(
		key.WithKeys("5"),
		key.WithHelp("5", "jobs"),
	),
	Refresh: key.NewBinding(
		key.WithKeys("r"),
		key.WithHelp("r", "refresh"),
	),
	RefreshAll: key.NewBinding(
		key.WithKeys("R"),
		key.WithHelp("R", "refresh all"),
	),
	Quit: key.NewBinding(
		key.WithKeys("q", "ctrl+c"),
		key.WithHelp("q", "quit"),
	),
	Help: key.NewBinding(
		key.WithKeys("?"),
		key.WithHelp("?", "help"),
	),
	Up: key.NewBinding(
		key.WithKeys("up", "k"),
		key.WithHelp("↑/k", "up"),
	),
	Down: key.NewBinding(
		key.WithKeys("down", "j"),
		key.WithHelp("↓/j", "down"),
	),
	GotoTop: key.NewBinding(
		key.WithKeys("g", "home"),
		key.WithHelp("g", "top"),
	),
	GotoBottom: key.NewBinding(
		key.WithKeys("G", "end"),
		key.WithHelp("G", "bottom"),
	),
	Search: key.NewBinding(
		key.WithKeys("/"),
		key.WithHelp("/", "search"),
	),
	Filter: key.NewBinding(
		key.WithKeys("f"),
		key.WithHelp("f", "filter"),
	),
	Tab: key.NewBinding(
		key.WithKeys("tab"),
		key.WithHelp("tab", "switch panel"),
	),
	Enter: key.NewBinding(
		key.WithKeys("enter"),
		key.WithHelp("enter", "select"),
	),
	Escape: key.NewBinding(
		key.WithKeys("esc"),
		key.WithHelp("esc", "back"),
	),
	NextPage: key.NewBinding(
		key.WithKeys("right", "l"),
		key.WithHelp("→/l", "next page"),
	),
	PrevPage: key.NewBinding(
		key.WithKeys("left", "h"),
		key.WithHelp("←/h", "prev page"),
	),
	Space: key.NewBinding(
		key.WithKeys(" "),
		key.WithHelp("space", "jump to top"),
	),
}

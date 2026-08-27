package admin

import (
	"database/sql"
	"fmt"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// ─── screen type ─────────────────────────────────────────────

type screen int

const (
	screenOverview screen = iota
	screenUsers
	screenActivity
	screenStorage
	screenJobs
)

var screenNames = map[screen]string{
	screenOverview: "Overview",
	screenUsers:    "Users",
	screenActivity: "Activity",
	screenStorage:  "Storage",
	screenJobs:     "Jobs",
}

// ─── tick message ─────────────────────────────────────────────

type tickMsg time.Time

func tickCmd(interval time.Duration) tea.Cmd {
	return tea.Tick(interval, func(t time.Time) tea.Msg {
		return tickMsg(t)
	})
}

// ─── root model ───────────────────────────────────────────────

type RootModel struct {
	db           *sql.DB
	dbPath       string
	activeScreen screen
	width        int
	height       int
	refreshSec   int
	lastRefresh  time.Time
	err          error
	showHelp     bool

	overview OverviewModel
	users    UsersModel
	activity ActivityModel
	storage  StorageModel
	jobs     JobsModel
}

func NewRootModel(db *sql.DB, dbPath string, refreshSec int) RootModel {
	return RootModel{
		db:          db,
		dbPath:      dbPath,
		refreshSec:  refreshSec,
		lastRefresh: time.Now(),
		overview:    NewOverviewModel(db),
		users:       NewUsersModel(db),
		activity:    NewActivityModel(db),
		storage:     NewStorageModel(db),
		jobs:        NewJobsModel(db),
	}
}

func (m RootModel) Init() tea.Cmd {
	return tea.Batch(
		m.overview.Init(),
		m.users.Init(),
		m.activity.Init(),
		m.storage.Init(),
		m.jobs.Init(),
		tickCmd(time.Duration(m.refreshSec)*time.Second),
	)
}

func (m RootModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) { //nolint:gocyclo,cyclop // message router
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		// Propagate to all children (they need dimensions for layout)
		childMsg := tea.WindowSizeMsg{
			Width:  msg.Width,
			Height: msg.Height - 3, // subtract tab bar + status bar
		}
		m.overview, _ = m.overview.Update(childMsg)
		m.users, _ = m.users.Update(childMsg)
		m.activity, _ = m.activity.Update(childMsg)
		m.storage, _ = m.storage.Update(childMsg)
		m.jobs, _ = m.jobs.Update(childMsg)
		return m, nil

	case tickMsg:
		m.lastRefresh = time.Now()
		var cmd tea.Cmd
		switch m.activeScreen {
		case screenOverview:
			cmd = m.overview.Refresh()
		case screenUsers:
			cmd = m.users.Refresh()
		case screenActivity:
			cmd = m.activity.Refresh()
		case screenStorage:
			cmd = m.storage.Refresh()
		case screenJobs:
			cmd = m.jobs.Refresh()
		}
		return m, tea.Batch(cmd, tickCmd(time.Duration(m.refreshSec)*time.Second))

	case tea.KeyMsg:
		// Help overlay swallows all keys except ?
		if m.showHelp {
			m.showHelp = false
			return m, nil
		}

		// Users search mode — pass keys to users model
		if m.activeScreen == screenUsers && m.users.typing {
			var cmd tea.Cmd
			m.users, cmd = m.users.Update(msg)
			return m, cmd
		}

		switch msg.String() {
		case "1":
			m.activeScreen = screenOverview
		case "2":
			m.activeScreen = screenUsers
		case "3":
			m.activeScreen = screenActivity
		case "4":
			m.activeScreen = screenStorage
		case "5":
			m.activeScreen = screenJobs
		case "r":
			m.lastRefresh = time.Now()
			var cmd tea.Cmd
			switch m.activeScreen {
			case screenOverview:
				cmd = m.overview.Refresh()
			case screenUsers:
				cmd = m.users.Refresh()
			case screenActivity:
				cmd = m.activity.Refresh()
			case screenStorage:
				cmd = m.storage.Refresh()
			case screenJobs:
				cmd = m.jobs.Refresh()
			}
			return m, cmd
		case "R":
			m.lastRefresh = time.Now()
			return m, tea.Batch(
				m.overview.Refresh(),
				m.users.Refresh(),
				m.activity.Refresh(),
				m.storage.Refresh(),
				m.jobs.Refresh(),
			)
		case "?":
			m.showHelp = !m.showHelp
			return m, nil
		case "q", "ctrl+c":
			return m, tea.Quit
		default:
			// Pass to active screen
			return m.passToActive(msg)
		}
		return m, nil

	// Route data messages to the appropriate child model
	case overviewDataMsg:
		var cmd tea.Cmd
		m.overview, cmd = m.overview.Update(msg)
		if msg.err != nil {
			m.err = msg.err
		} else {
			m.err = nil
		}
		return m, cmd

	case usersDataMsg:
		var cmd tea.Cmd
		m.users, cmd = m.users.Update(msg)
		return m, cmd

	case searchDebounceMsg:
		if m.users.search == msg.query {
			return m, m.users.loadCmd()
		}
		return m, nil

	case activityDataMsg:
		var cmd tea.Cmd
		m.activity, cmd = m.activity.Update(msg)
		return m, cmd

	case storageDataMsg:
		var cmd tea.Cmd
		m.storage, cmd = m.storage.Update(msg)
		return m, cmd

	case jobsDataMsg:
		var cmd tea.Cmd
		m.jobs, cmd = m.jobs.Update(msg)
		return m, cmd
	}

	return m, nil
}

func (m RootModel) passToActive(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch m.activeScreen {
	case screenOverview:
		var cmd tea.Cmd
		m.overview, cmd = m.overview.Update(msg)
		return m, cmd
	case screenUsers:
		var cmd tea.Cmd
		m.users, cmd = m.users.Update(msg)
		return m, cmd
	case screenActivity:
		var cmd tea.Cmd
		m.activity, cmd = m.activity.Update(msg)
		return m, cmd
	case screenStorage:
		var cmd tea.Cmd
		m.storage, cmd = m.storage.Update(msg)
		return m, cmd
	case screenJobs:
		var cmd tea.Cmd
		m.jobs, cmd = m.jobs.Update(msg)
		return m, cmd
	}
	return m, nil
}

func (m RootModel) View() string {
	tabBar := m.renderTabBar()
	statusBar := m.renderStatusBar()

	contentH := max(
		// 1 tab + 1 blank + 1 status
		m.height-3, 1)

	var content string
	switch m.activeScreen {
	case screenOverview:
		content = m.overview.View()
	case screenUsers:
		content = m.users.View()
	case screenActivity:
		content = m.activity.View()
	case screenStorage:
		content = m.storage.View()
	case screenJobs:
		content = m.jobs.View()
	}

	// Pad content to fill available height
	lines := strings.Split(content, "\n")
	for len(lines) < contentH {
		lines = append(lines, "")
	}
	if len(lines) > contentH {
		lines = lines[:contentH]
	}
	content = strings.Join(lines, "\n")

	view := lipgloss.JoinVertical(lipgloss.Left,
		tabBar,
		content,
		statusBar,
	)

	if m.showHelp {
		return lipgloss.Place(m.width, m.height,
			lipgloss.Center, lipgloss.Center,
			m.renderHelp(),
			lipgloss.WithWhitespaceChars(" "),
			lipgloss.WithWhitespaceForeground(DimColor),
		)
	}

	return view
}

// ─── tab bar ─────────────────────────────────────────────────

func (m RootModel) renderTabBar() string {
	tabs := []struct {
		s   screen
		num string
	}{
		{screenOverview, "1"},
		{screenUsers, "2"},
		{screenActivity, "3"},
		{screenStorage, "4"},
		{screenJobs, "5"},
	}

	var parts []string
	for _, t := range tabs {
		label := fmt.Sprintf(" %s %s ", t.num, screenNames[t.s])
		if t.s == m.activeScreen {
			parts = append(parts, TabActiveStyle.Render(label))
		} else {
			parts = append(parts, TabInactiveStyle.Render(label))
		}
	}
	return lipgloss.JoinHorizontal(lipgloss.Left, parts...)
}

// ─── status bar ──────────────────────────────────────────────

func (m RootModel) renderStatusBar() string {
	var dbName string
	if len(m.dbPath) > 40 {
		dbName = "…" + m.dbPath[len(m.dbPath)-37:]
	} else {
		dbName = m.dbPath
	}

	elapsed := time.Since(m.lastRefresh)
	var timeStr string
	switch {
	case elapsed < time.Minute:
		timeStr = fmt.Sprintf("refreshed %ds ago", int(elapsed.Seconds()))
	default:
		timeStr = fmt.Sprintf("refreshed %dm ago", int(elapsed.Minutes()))
	}

	var statusContent string
	if m.err != nil {
		statusContent = ErrorStyle.Render("⚠ " + m.err.Error())
	} else {
		statusContent = MutedStyle.Render(
			fmt.Sprintf("damask-admin  •  %s  •  %s  •  [r] refresh  [?] help  [q] quit", dbName, timeStr),
		)
	}

	return StatusBarStyle.Width(m.width).Render(statusContent)
}

// ─── help overlay ────────────────────────────────────────────

func (m RootModel) renderHelp() string {
	nav := lipgloss.JoinVertical(lipgloss.Left,
		lipgloss.NewStyle().Bold(true).Foreground(ColorMuted).Render("Navigation"),
		DividerStyle.Render("─────────"),
		HelpStyle.Render("1–5    Switch screen"),
		HelpStyle.Render("↑/k    Scroll up"),
		HelpStyle.Render("↓/j    Scroll down"),
		HelpStyle.Render("g      Jump to top"),
		HelpStyle.Render("G      Jump to bottom"),
		HelpStyle.Render("←/→    Prev/next page"),
	)
	actions := lipgloss.JoinVertical(lipgloss.Left,
		lipgloss.NewStyle().Bold(true).Foreground(ColorMuted).Render("Actions"),
		DividerStyle.Render("───────"),
		HelpStyle.Render("r   Refresh current"),
		HelpStyle.Render("R   Refresh all"),
		HelpStyle.Render("/   Search (Users)"),
		HelpStyle.Render("f   Filter (Activity)"),
		HelpStyle.Render("Tab Switch panel (Jobs)"),
		HelpStyle.Render("?   Toggle help"),
		HelpStyle.Render("q   Quit"),
	)

	content := lipgloss.JoinHorizontal(lipgloss.Top,
		nav, "    ", actions,
	)

	title := ModalTitleStyle.Render("Keyboard shortcuts")
	return ModalStyle.Render(lipgloss.JoinVertical(lipgloss.Left, title, content))
}

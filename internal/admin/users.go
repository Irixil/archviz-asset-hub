package admin

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/table"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

const (
	usersPageSize = 50
	keyEnter      = "enter"
)

// ─── messages ────────────────────────────────────────────────

type usersDataMsg struct {
	rows  []UserRow
	total int
	err   error
}

// ─── model ───────────────────────────────────────────────────

type UsersModel struct {
	db       *sql.DB
	width    int
	height   int
	table    table.Model
	rows     []UserRow
	total    int
	page     int
	orderBy  string
	orderDir string
	search   string
	typing   bool
	err      error
	loading  bool
}

func NewUsersModel(db *sql.DB) UsersModel {
	t := table.New(
		table.WithFocused(true),
		table.WithHeight(20),
	)
	s := table.DefaultStyles()
	s.Header = TableHeaderStyle
	s.Selected = TableSelectedStyle
	s.Cell = TableCellStyle
	t.SetStyles(s)

	return UsersModel{
		db:       db,
		table:    t,
		orderBy:  "u.created_at",
		orderDir: "DESC",
		loading:  true,
	}
}

func (m UsersModel) Init() tea.Cmd {
	return m.loadCmd()
}

func (m UsersModel) Update(msg tea.Msg) (UsersModel, tea.Cmd) {
	switch msg := msg.(type) {
	case usersDataMsg:
		m.loading = false
		m.err = msg.err
		if msg.err == nil {
			m.rows = msg.rows
			m.total = msg.total
			m = m.rebuildTable()
		}

	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m = m.rebuildCols()

	case tea.KeyMsg:
		if m.typing {
			switch msg.String() {
			case "esc":
				m.typing = false
				m.search = ""
				m.page = 0
				return m, m.loadCmd()
			case keyEnter:
				m.typing = false
				m.page = 0
				return m, m.loadCmd()
			case "backspace":
				if len(m.search) > 0 {
					m.search = m.search[:len(m.search)-1]
					return m, m.debounceSearchCmd()
				}
			default:
				if len(msg.String()) == 1 {
					m.search += msg.String()
					return m, m.debounceSearchCmd()
				}
			}
			return m, nil
		}

		switch msg.String() {
		case "/":
			m.typing = true
			return m, nil
		case "right", "l":
			if (m.page+1)*usersPageSize < m.total {
				m.page++
				return m, m.loadCmd()
			}
		case "left", "h":
			if m.page > 0 {
				m.page--
				return m, m.loadCmd()
			}
		case "r":
			return m, m.loadCmd()
		}
	}

	var cmd tea.Cmd
	m.table, cmd = m.table.Update(msg)
	return m, cmd
}

func (m UsersModel) Refresh() tea.Cmd { return m.loadCmd() }

func (m UsersModel) View() string {
	if m.loading {
		return MutedStyle.Render("  Loading users...")
	}
	if m.err != nil {
		return viewCenterErr(m.err, m.width, m.height)
	}

	var sb strings.Builder

	// Header row with pagination info
	start := m.page*usersPageSize + 1
	end := max(start+len(m.rows)-1, start)
	pagination := MutedStyle.Render(fmt.Sprintf("Showing %d–%d of %d", start, end, m.total))
	var filterInfo string
	if m.search != "" {
		filterInfo = "  " + SearchPromptStyle.Render("filter: ") + `"` + m.search + `"`
	}
	if m.typing {
		filterInfo = "  " + SearchPromptStyle.Render("/") + m.search + "▌"
	}
	sb.WriteString(lipgloss.JoinHorizontal(lipgloss.Left, pagination, filterInfo))
	sb.WriteString("\n")
	sb.WriteString(m.table.View())
	sb.WriteString("\n")

	hints := MutedStyle.Render("/ search   ←/→ page   ↑/↓ navigate")
	sb.WriteString(hints)

	return sb.String()
}

func (m UsersModel) rebuildCols() UsersModel {
	narrow := m.width < 100
	var cols []table.Column
	if narrow {
		cols = []table.Column{
			{Title: "Email", Width: clamp(m.width-30, 20, 40)},
			{Title: "Workspace", Width: clamp(m.width-40, 10, 25)},
			{Title: "Role", Width: 8},
			{Title: "Assets", Width: 7},
			{Title: "Joined", Width: 12},
		}
	} else {
		emailW := clamp(m.width-90, 20, 35)
		wsW := clamp(m.width-90, 15, 25)
		cols = []table.Column{
			{Title: "Email", Width: emailW},
			{Title: "Workspace", Width: wsW},
			{Title: "Role", Width: 8},
			{Title: "Assets", Width: 7},
			{Title: "Last upload", Width: 14},
			{Title: "Joined", Width: 12},
		}
	}
	m.table.SetColumns(cols)
	m.table.SetHeight(m.height - 6)
	return m
}

func (m UsersModel) rebuildTable() UsersModel {
	m = m.rebuildCols()
	narrow := m.width < 100
	var rows []table.Row
	for _, r := range m.rows {
		var lastUp string
		if r.LastUpload != nil {
			lastUp = timeAgo(*r.LastUpload)
		} else {
			lastUp = "—"
		}
		joined := r.CreatedAt.Format("Jan 02 2006")
		if narrow {
			rows = append(rows, table.Row{
				truncate(r.Email, 40),
				truncate(r.WorkspaceName, 25),
				r.Role,
				commaSep(r.AssetCount),
				joined,
			})
		} else {
			rows = append(rows, table.Row{
				truncate(r.Email, 35),
				truncate(r.WorkspaceName, 25),
				r.Role,
				commaSep(r.AssetCount),
				lastUp,
				joined,
			})
		}
	}
	m.table.SetRows(rows)
	return m
}

func (m UsersModel) loadCmd() tea.Cmd {
	db := m.db
	page := m.page
	search := m.search
	orderBy := m.orderBy
	orderDir := m.orderDir
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		rows, total, err := QueryUsers(ctx, db, usersPageSize, page*usersPageSize, search, orderBy, orderDir)
		return usersDataMsg{rows: rows, total: total, err: err}
	}
}

type searchDebounceMsg struct{ query string }

func (m UsersModel) debounceSearchCmd() tea.Cmd {
	q := m.search
	return tea.Tick(300*time.Millisecond, func(_ time.Time) tea.Msg {
		return searchDebounceMsg{query: q}
	})
}

// ─── helpers ─────────────────────────────────────────────────

func timeAgo(t time.Time) string {
	d := time.Since(t)
	switch {
	case d < time.Minute:
		return "just now"
	case d < time.Hour:
		return fmt.Sprintf("%d min ago", int(d.Minutes()))
	case d < 24*time.Hour:
		return fmt.Sprintf("%d hr ago", int(d.Hours()))
	case d < 48*time.Hour:
		return "yesterday"
	case d < 7*24*time.Hour:
		return fmt.Sprintf("%d days ago", int(d.Hours()/24))
	default:
		return t.Format("Jan 02 2006")
	}
}

func truncate(s string, maxLength int) string {
	runes := []rune(s)
	if len(runes) <= maxLength {
		return s
	}
	return string(runes[:maxLength-1]) + "…"
}

package admin

import (
	"context"
	"database/sql"
	"strconv"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/table"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// ─── messages ────────────────────────────────────────────────

type jobsDataMsg struct {
	health []JobRow
	failed []JobDetailRow
	err    error
}

// ─── panels ──────────────────────────────────────────────────

type jobsPanel int

const (
	panelHealth jobsPanel = iota
	panelFailed
)

// ─── model ───────────────────────────────────────────────────

type JobsModel struct {
	db          *sql.DB
	width       int
	height      int
	health      []JobRow
	failed      []JobDetailRow
	failedTable table.Model
	activePanel jobsPanel
	expandedJob *JobDetailRow
	err         error
	loading     bool
}

func NewJobsModel(db *sql.DB) JobsModel {
	t := table.New(table.WithFocused(true), table.WithHeight(8))
	s := table.DefaultStyles()
	s.Header = TableHeaderStyle
	s.Selected = TableSelectedStyle
	s.Cell = TableCellStyle
	t.SetStyles(s)

	return JobsModel{
		db:          db,
		failedTable: t,
		loading:     true,
	}
}

func (m JobsModel) Init() tea.Cmd {
	return loadJobsCmd(m.db)
}

func (m JobsModel) Update(msg tea.Msg) (JobsModel, tea.Cmd) {
	switch msg := msg.(type) {
	case jobsDataMsg:
		m.loading = false
		m.err = msg.err
		if msg.err == nil {
			m.health = msg.health
			m.failed = msg.failed
			m = m.rebuildFailedTable()
		}

	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m = m.rebuildFailedCols()

	case tea.KeyMsg:
		return handleJobsKey(m, msg)
	}

	var cmd tea.Cmd
	m.failedTable, cmd = m.failedTable.Update(msg)
	return m, cmd
}

func handleJobsKey(m JobsModel, msg tea.KeyMsg) (JobsModel, tea.Cmd) {
	if m.expandedJob != nil {
		switch msg.String() {
		case "esc", "enter", "q":
			m.expandedJob = nil
		}
		return m, nil
	}
	switch msg.String() {
	case "tab":
		if m.activePanel == panelHealth {
			m.activePanel = panelFailed
			m.failedTable.Focus()
		} else {
			m.activePanel = panelHealth
			m.failedTable.Blur()
		}
	case "enter":
		if m.activePanel == panelFailed {
			if selected := m.failedTable.SelectedRow(); selected != nil {
				for i := range m.failed {
					if m.failed[i].ID == selected[0] {
						m.expandedJob = &m.failed[i]
						break
					}
				}
			}
		}
	case "r":
		return m, m.Refresh()
	}
	var cmd tea.Cmd
	m.failedTable, cmd = m.failedTable.Update(msg)
	return m, cmd
}

func (m JobsModel) Refresh() tea.Cmd { return loadJobsCmd(m.db) }

func (m JobsModel) View() string {
	if m.loading {
		return MutedStyle.Render("  Loading jobs...")
	}
	if m.err != nil {
		return viewCenterErr(m.err, m.width, m.height)
	}

	var sb strings.Builder
	sb.WriteString(renderSectionDivider("Job queue health", m.width))
	sb.WriteString("\n")
	sb.WriteString(renderJobHealthTable(m.health, m.width, m.activePanel == panelHealth))
	sb.WriteString("\n")
	sb.WriteString(renderSectionDivider("Failed jobs", m.width))
	sb.WriteString("\n")
	if len(m.failed) == 0 {
		sb.WriteString(SuccessStyle.Render("  No failed jobs ✓"))
	} else {
		sb.WriteString(m.failedTable.View())
		sb.WriteString("\n")
		sb.WriteString(MutedStyle.Render("enter: expand error   tab: switch panel"))
	}

	view := sb.String()

	// Render modal overlay if a job is expanded
	if m.expandedJob != nil {
		overlay := m.renderJobModal()
		return lipgloss.Place(m.width, m.height,
			lipgloss.Center, lipgloss.Center,
			overlay,
			lipgloss.WithWhitespaceChars(" "),
			lipgloss.WithWhitespaceForeground(DimColor),
		)
	}

	return view
}

func renderJobHealthTable(rows []JobRow, width int, _ bool) string {
	if len(rows) == 0 {
		return MutedStyle.Render("  No jobs in queue")
	}

	typeW := clamp(width-60, 15, 30)
	hdr := lipgloss.JoinHorizontal(lipgloss.Left,
		TableHeaderStyle.Width(typeW).Render("Type"),
		TableHeaderStyle.Width(10).Render("pending"),
		TableHeaderStyle.Width(12).Render("processing"),
		TableHeaderStyle.Width(8).Render("done"),
		TableHeaderStyle.Width(8).Render("failed"),
	)
	var lines []string
	lines = append(lines, hdr)
	lines = append(lines, DividerStyle.Render(strings.Repeat("─", clamp(width-2, 40, 80))))

	// Group by type
	type jobTypeSummary struct {
		pending    int
		processing int
		done       int
		failed     int
		oldestSec  int
		lastError  string
	}
	byType := make(map[string]*jobTypeSummary)
	var order []string
	for _, r := range rows {
		if _, ok := byType[r.Type]; !ok {
			byType[r.Type] = &jobTypeSummary{}
			order = append(order, r.Type)
		}
		switch r.Status {
		case "pending":
			byType[r.Type].pending = r.Count
		case "processing":
			byType[r.Type].processing = r.Count
			if r.OldestSec > byType[r.Type].oldestSec {
				byType[r.Type].oldestSec = r.OldestSec
			}
		case "done":
			byType[r.Type].done = r.Count
		case "failed":
			byType[r.Type].failed = r.Count
			byType[r.Type].lastError = r.LastError
		}
	}

	for _, typ := range order {
		s := byType[typ]
		failedStr := strconv.Itoa(s.failed)
		failedStyle := lipgloss.NewStyle().Width(8)
		if s.failed > 0 {
			failedStyle = failedStyle.Bold(true).Foreground(ColorDanger)
		}

		procStr := strconv.Itoa(s.processing)
		procStyle := lipgloss.NewStyle().Width(12)
		if s.processing > 0 {
			procStyle = procStyle.Foreground(ColorWarning)
		}

		warning := ""
		if s.processing > 0 && s.oldestSec > 300 {
			warning = " " + WarningStyle.Render("⚠")
		}

		line := lipgloss.JoinHorizontal(lipgloss.Left,
			lipgloss.NewStyle().Width(typeW).Render(truncate(typ, typeW)),
			lipgloss.NewStyle().Width(10).Render(strconv.Itoa(s.pending)),
			procStyle.Render(procStr+warning),
			lipgloss.NewStyle().Width(8).Render(commaSep(s.done)),
			failedStyle.Render(failedStr),
		)
		lines = append(lines, line)
	}

	return strings.Join(lines, "\n")
}

func (m JobsModel) rebuildFailedCols() JobsModel {
	errW := clamp(m.width-70, 20, 50)
	cols := []table.Column{
		{Title: "ID", Width: 14},
		{Title: "Type", Width: 16},
		{Title: "Attempts", Width: 9},
		{Title: "Age", Width: 12},
		{Title: "Error", Width: errW},
	}
	m.failedTable.SetColumns(cols)
	m.failedTable.SetHeight(clamp(m.height/3, 5, 12))
	return m
}

func (m JobsModel) rebuildFailedTable() JobsModel {
	m = m.rebuildFailedCols()
	errW := clamp(m.width-70, 20, 50)
	var rows []table.Row
	for _, r := range m.failed {
		rows = append(rows, table.Row{
			r.ID,
			truncate(r.Type, 16),
			strconv.Itoa(r.Attempts),
			timeAgo(r.UpdatedAt),
			truncate(r.Error, errW),
		})
	}
	m.failedTable.SetRows(rows)
	return m
}

func (m JobsModel) renderJobModal() string {
	j := m.expandedJob
	var sb strings.Builder
	sb.WriteString(ModalTitleStyle.Render("Job detail"))
	sb.WriteString("\n")

	field := func(k, v string) string {
		return lipgloss.JoinHorizontal(lipgloss.Left,
			ModalKeyStyle.Render(k+":"),
			ModalValueStyle.Render(v),
		)
	}

	sb.WriteString(field("ID", j.ID))
	sb.WriteString("\n")
	sb.WriteString(field("Type", j.Type))
	sb.WriteString("\n")
	sb.WriteString(field("Status", j.Status))
	sb.WriteString("\n")
	sb.WriteString(field("Attempts", strconv.Itoa(j.Attempts)))
	sb.WriteString("\n")
	sb.WriteString(field("Created", j.CreatedAt.Format("2006-01-02 15:04:05")))
	sb.WriteString("\n")
	sb.WriteString(field("Updated", j.UpdatedAt.Format("2006-01-02 15:04:05")))
	sb.WriteString("\n\n")

	sb.WriteString(ModalKeyStyle.Render("Payload:"))
	sb.WriteString("\n")
	sb.WriteString(ModalValueStyle.Width(58).Render(wordWrap(j.Payload, 58)))
	sb.WriteString("\n\n")

	sb.WriteString(ModalKeyStyle.Render("Error:"))
	sb.WriteString("\n")
	sb.WriteString(ErrorStyle.Width(58).Render(wordWrap(j.Error, 58)))
	sb.WriteString("\n\n")

	sb.WriteString(MutedStyle.Render("[Esc] close"))

	return ModalStyle.Render(sb.String())
}

func wordWrap(s string, maxWidth int) string {
	if len(s) <= maxWidth {
		return s
	}
	var lines []string
	for len(s) > maxWidth {
		lines = append(lines, s[:maxWidth])
		s = s[maxWidth:]
	}
	if len(s) > 0 {
		lines = append(lines, s)
	}
	return strings.Join(lines, "\n")
}

// ─── async loader ────────────────────────────────────────────

func loadJobsCmd(db *sql.DB) tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		health, err := QueryJobHealth(ctx, db)
		if err != nil {
			return jobsDataMsg{err: err}
		}
		failed, err := QueryFailedJobs(ctx, db, 10)
		if err != nil {
			return jobsDataMsg{err: err}
		}
		return jobsDataMsg{health: health, failed: failed}
	}
}

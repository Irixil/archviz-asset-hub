package admin

import (
	"context"
	"database/sql"
	"fmt"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// ─── messages ────────────────────────────────────────────────

type overviewDataMsg struct {
	stats   OverviewStats
	signups []DayCount
	uploads []DayCount
	err     error
}

// ─── model ───────────────────────────────────────────────────

type OverviewModel struct {
	db      *sql.DB
	width   int
	height  int
	stats   OverviewStats
	signups []DayCount
	uploads []DayCount
	err     error
	loading bool
}

func NewOverviewModel(db *sql.DB) OverviewModel {
	return OverviewModel{db: db, loading: true}
}

func (m OverviewModel) Init() tea.Cmd {
	return loadOverviewCmd(m.db)
}

func (m OverviewModel) Update(msg tea.Msg) (OverviewModel, tea.Cmd) {
	switch msg := msg.(type) {
	case overviewDataMsg:
		m.loading = false
		m.err = msg.err
		if msg.err == nil {
			m.stats = msg.stats
			m.signups = msg.signups
			m.uploads = msg.uploads
		}
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
	}
	return m, nil
}

func (m OverviewModel) Refresh() tea.Cmd { return loadOverviewCmd(m.db) }

func (m OverviewModel) View() string {
	if m.loading {
		return MutedStyle.Render("  Loading overview...")
	}
	if m.err != nil {
		return viewCenterErr(m.err, m.width, m.height)
	}

	s := m.stats
	narrow := m.width < 100

	row1 := []string{
		renderStatBox("total users", strconv.Itoa(s.TotalUsers), false, false),
		renderStatBox("new today", fmt.Sprintf("+%d", s.NewUsersToday), false, false),
		renderStatBox("total assets", commaSep(s.TotalAssets), false, false),
		renderStatBox("storage", formatMB(s.TotalStorageMB), false, false),
	}
	row2 := []string{
		renderStatBox("this week", fmt.Sprintf("+%d", s.NewUsersThisWeek), false, false),
		renderStatBox("active workspaces", strconv.Itoa(s.ActiveWorkspaces), false, false),
		renderStatBox("failed jobs", strconv.Itoa(s.JobsFailed), s.JobsFailed > 0, false),
		renderStatBox("processing", strconv.Itoa(s.JobsProcessing), false, s.JobsProcessing > 0),
	}

	var lines []string
	if narrow {
		lines = append(lines,
			lipgloss.JoinHorizontal(lipgloss.Top, row1[0], "  ", row1[1]),
			"",
			lipgloss.JoinHorizontal(lipgloss.Top, row1[2], "  ", row1[3]),
			"",
			lipgloss.JoinHorizontal(lipgloss.Top, row2[0], "  ", row2[1]),
			"",
			lipgloss.JoinHorizontal(lipgloss.Top, row2[2], "  ", row2[3]),
		)
	} else {
		lines = append(lines,
			lipgloss.JoinHorizontal(lipgloss.Top, row1[0], "  ", row1[1], "  ", row1[2], "  ", row1[3]),
			"",
			lipgloss.JoinHorizontal(lipgloss.Top, row2[0], "  ", row2[1], "  ", row2[2], "  ", row2[3]),
		)
	}

	lines = append(lines, "", renderSectionDivider("Signups this week", m.width))
	lines = append(lines, renderSparkline(m.signups))
	lines = append(lines, "", renderSectionDivider("Uploads this week", m.width))
	lines = append(lines, renderSparkline(m.uploads))

	return strings.Join(lines, "\n")
}

// ─── rendering helpers ───────────────────────────────────────

func renderStatBox(label, value string, danger, warning bool) string {
	valStyle := StatValueStyle
	boxStyle := StatBoxStyle
	if danger {
		valStyle = StatValueDangerStyle
		boxStyle = StatBoxDangerStyle
	} else if warning {
		valStyle = StatValueWarningStyle
		boxStyle = StatBoxWarningStyle
	}
	inner := lipgloss.JoinVertical(lipgloss.Center,
		valStyle.Width(16).Render(value),
		StatLabelStyle.Width(16).Render(label),
	)
	return boxStyle.Render(inner)
}

var barBlocks = []rune{'▁', '▂', '▃', '▄', '▅', '▆', '▇', '█'}

func blockChar(value, maxBlock int) rune {
	if maxBlock == 0 {
		return barBlocks[0]
	}
	idx := int(float64(value) / float64(maxBlock) * float64(len(barBlocks)-1))
	if idx >= len(barBlocks) {
		idx = len(barBlocks) - 1
	}
	return barBlocks[idx]
}

func renderSparkline(days []DayCount) string {
	if len(days) == 0 {
		return MutedStyle.Render("  No data")
	}
	maxVal := 0
	dayMap := make(map[string]int)
	for _, d := range days {
		dayMap[d.Day] = d.Count
		if d.Count > maxVal {
			maxVal = d.Count
		}
	}

	now := time.Now()
	var lines []string
	for i := 6; i >= 0; i-- {
		day := now.AddDate(0, 0, -i)
		key := day.Format("2006-01-02")
		count := dayMap[key]
		ch := blockChar(count, maxVal)
		barLen := 1
		if maxVal > 0 {
			barLen = clamp(count*16/maxVal, 1, 16)
		}
		bar := strings.Repeat(string(ch), barLen)
		suffix := ""
		if i == 0 {
			suffix = MutedStyle.Render(" (today)")
		}
		line := fmt.Sprintf("  %-3s  %s  %d%s", day.Format("Mon"), SuccessStyle.Render(bar), count, suffix)
		lines = append(lines, line)
	}
	return strings.Join(lines, "\n")
}

func renderSectionDivider(title string, width int) string {
	titleStr := SectionTitleStyle.Render(title)
	titleLen := utf8.RuneCountInString(title)
	dashCount := max(width-titleLen-2, 2)
	dashes := DividerStyle.Render(strings.Repeat("─", dashCount))
	return titleStr + " " + dashes
}

func viewCenterErr(err error, w, h int) string {
	msg := ErrorStyle.Render("⚠ " + err.Error())
	hint := MutedStyle.Render("[r] retry")
	content := lipgloss.JoinVertical(lipgloss.Center, msg, hint)
	return lipgloss.Place(w, h, lipgloss.Center, lipgloss.Center, content)
}

func commaSep(n int) string {
	s := strconv.Itoa(n)
	if n < 1000 {
		return s
	}
	var result []byte
	for i := range s {
		if i > 0 && (len(s)-i)%3 == 0 {
			result = append(result, ',')
		}
		result = append(result, s[i])
	}
	return string(result)
}

func formatMB(mb float64) string {
	switch {
	case mb >= 1024*1024:
		return fmt.Sprintf("%.1f TB", mb/1024/1024)
	case mb >= 1024:
		return fmt.Sprintf("%.1f GB", mb/1024)
	default:
		return fmt.Sprintf("%.1f MB", mb)
	}
}

func clamp(v, lo, hi int) int {
	if v < lo {
		return lo
	}
	if v > hi {
		return hi
	}
	return v
}

// ─── async loader ────────────────────────────────────────────

func loadOverviewCmd(db *sql.DB) tea.Cmd {
	return func() tea.Msg {
		ctx := context.Background()
		stats, err := QueryOverviewStats(ctx, db)
		if err != nil {
			return overviewDataMsg{err: err}
		}
		signups, err := QuerySignupsByDay(ctx, db)
		if err != nil {
			return overviewDataMsg{err: err}
		}
		uploads, err := QueryUploadsByDay(ctx, db)
		if err != nil {
			return overviewDataMsg{err: err}
		}
		return overviewDataMsg{stats: stats, signups: signups, uploads: uploads}
	}
}

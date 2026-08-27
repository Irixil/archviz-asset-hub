package admin

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// ─── messages ────────────────────────────────────────────────

type storageDataMsg struct {
	rows    []StorageRow
	largest LargestAsset
	err     error
}

// ─── model ───────────────────────────────────────────────────

type StorageModel struct {
	db      *sql.DB
	width   int
	height  int
	rows    []StorageRow
	largest LargestAsset
	cursor  int
	err     error
	loading bool
}

func NewStorageModel(db *sql.DB) StorageModel {
	return StorageModel{db: db, loading: true}
}

func (m StorageModel) Init() tea.Cmd {
	return loadStorageCmd(m.db)
}

func (m StorageModel) Update(msg tea.Msg) (StorageModel, tea.Cmd) {
	switch msg := msg.(type) {
	case storageDataMsg:
		m.loading = false
		m.err = msg.err
		if msg.err == nil {
			m.rows = msg.rows
			m.largest = msg.largest
		}
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
	case tea.KeyMsg:
		switch msg.String() {
		case "up", "k":
			if m.cursor > 0 {
				m.cursor--
			}
		case "down", "j":
			if m.cursor < len(m.rows)-1 {
				m.cursor++
			}
		case "g", "home":
			m.cursor = 0
		case "G", "end":
			m.cursor = len(m.rows) - 1
		case "r":
			return m, m.Refresh()
		}
	}
	return m, nil
}

func (m StorageModel) Refresh() tea.Cmd { return loadStorageCmd(m.db) }

func (m StorageModel) View() string {
	if m.loading {
		return MutedStyle.Render("  Loading storage...")
	}
	if m.err != nil {
		return viewCenterErr(m.err, m.width, m.height)
	}

	var sb strings.Builder
	sb.WriteString(renderSectionDivider("Storage breakdown", m.width))
	sb.WriteString("\n")

	// Compute max for bar chart
	maxMB := 0.0
	totalMB := 0.0
	for _, r := range m.rows {
		if r.TotalMB > maxMB {
			maxMB = r.TotalMB
		}
		totalMB += r.TotalMB
	}

	// Column widths
	wsW := clamp(m.width-65, 15, 30)
	nameHdr := lipgloss.NewStyle().Bold(true).Foreground(ColorMuted).Width(wsW).Render("Workspace")
	hdr := lipgloss.JoinHorizontal(lipgloss.Left,
		nameHdr,
		TableHeaderStyle.Width(8).Render("Assets"),
		TableHeaderStyle.Width(10).Render("Versions"),
		TableHeaderStyle.Width(12).Render("Total"),
		TableHeaderStyle.Width(22).Render("Usage"),
	)
	sb.WriteString(hdr)
	sb.WriteString("\n")
	sb.WriteString(DividerStyle.Render(strings.Repeat("─", clamp(m.width-2, 40, 90))))
	sb.WriteString("\n")

	// Rows
	visibleH := m.height - 10
	start := 0
	if m.cursor >= visibleH {
		start = m.cursor - visibleH + 1
	}

	for i, r := range m.rows {
		if i < start || i >= start+visibleH {
			continue
		}
		bar := storageBar(r.TotalMB, maxMB, totalMB)
		wsName := truncate(r.WorkspaceName, wsW)
		wsStyle := lipgloss.NewStyle().Width(wsW)
		if i == m.cursor {
			wsStyle = wsStyle.Bold(true).Foreground(ColorPrimary)
		}
		line := lipgloss.JoinHorizontal(lipgloss.Left,
			wsStyle.Render(wsName),
			lipgloss.NewStyle().Width(8).Render(commaSep(r.AssetCount)),
			lipgloss.NewStyle().Width(10).Render(commaSep(r.VersionCount)),
			lipgloss.NewStyle().Width(12).Render(formatMB(r.TotalMB)),
			bar,
		)
		sb.WriteString(line)
		sb.WriteString("\n")
	}

	// Total row
	sb.WriteString(DividerStyle.Render(strings.Repeat("─", 30)))
	sb.WriteString("\n")
	totalLine := lipgloss.JoinHorizontal(lipgloss.Left,
		lipgloss.NewStyle().Width(wsW+8+10).Render(""),
		lipgloss.NewStyle().Bold(true).Foreground(ColorPrimary).Width(12).Render(formatMB(totalMB)),
		MutedStyle.Render("total"),
	)
	sb.WriteString(totalLine)
	sb.WriteString("\n\n")

	// Summary lines
	if m.largest.Filename != "" {
		sb.WriteString(MutedStyle.Render(fmt.Sprintf("Largest single asset: %s — %s (%s)",
			formatMB(m.largest.SizeMB),
			truncate(m.largest.Filename, 40),
			m.largest.WorkspaceName,
		)))
		sb.WriteString("\n")
	}

	// Oldest asset across all workspaces
	var oldest *time.Time
	for _, r := range m.rows {
		if r.OldestAsset != nil {
			if oldest == nil || r.OldestAsset.Before(*oldest) {
				oldest = r.OldestAsset
			}
		}
	}
	if oldest != nil {
		sb.WriteString(MutedStyle.Render(fmt.Sprintf("Oldest asset: %s", oldest.Format("Jan 2 2006"))))
		sb.WriteString("\n")
	}

	return sb.String()
}

func storageBar(mb, maxMB, totalMB float64) string {
	const barWidth = 20
	filled := 0
	if maxMB > 0 {
		filled = int(mb / maxMB * barWidth)
	}
	if filled > barWidth {
		filled = barWidth
	}
	bar := strings.Repeat("█", filled) + strings.Repeat("░", barWidth-filled)

	// Color based on proportion of total
	var barStyle lipgloss.Style
	proportion := 0.0
	if totalMB > 0 {
		proportion = mb / totalMB
	}
	switch {
	case proportion > 0.75:
		barStyle = lipgloss.NewStyle().Foreground(ColorDanger)
	case proportion > 0.25:
		barStyle = lipgloss.NewStyle().Foreground(ColorWarning)
	default:
		barStyle = lipgloss.NewStyle().Foreground(ColorSuccess)
	}
	return barStyle.Render(bar)
}

// ─── async loader ────────────────────────────────────────────

func loadStorageCmd(db *sql.DB) tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		rows, err := QueryStorageBreakdown(ctx, db)
		if err != nil {
			return storageDataMsg{err: err}
		}
		largest, err := QueryLargestAsset(ctx, db)
		if err != nil {
			return storageDataMsg{err: err}
		}
		return storageDataMsg{rows: rows, largest: largest}
	}
}

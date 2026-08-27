// Package admin provides admin-only handlers and reporting.
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

const (
	activityLimit             = 200
	activityFilterSystemActor = "system_actor"
	activityEventAssetCreated = "asset_created"
	activityEventAssetDeleted = "asset_deleted"
)

// ─── messages ────────────────────────────────────────────────

type activityDataMsg struct {
	rows []ActivityRow
	err  error
}

// ─── filter cycle ────────────────────────────────────────────

type activityFilter int

const (
	filterAll activityFilter = iota
	filterUploads
	filterChanges
	filterDeletes
	filterSystem
)

var filterLabels = map[activityFilter]string{
	filterAll:     "All",
	filterUploads: "Uploads",
	filterChanges: "Changes",
	filterDeletes: "Deletes",
	filterSystem:  "System",
}

var filterEventTypes = map[activityFilter]string{
	filterAll:     "",
	filterUploads: activityEventAssetCreated,
	filterChanges: "asset_renamed",
	filterDeletes: activityEventAssetDeleted,
	filterSystem:  "",
}

// ─── model ───────────────────────────────────────────────────

type ActivityModel struct {
	db         *sql.DB
	width      int
	height     int
	rows       []ActivityRow
	cursor     int
	filter     activityFilter
	autoScroll bool
	newCount   int
	err        error
	loading    bool
}

func NewActivityModel(db *sql.DB) ActivityModel {
	return ActivityModel{
		db:         db,
		autoScroll: true,
		loading:    true,
	}
}

func (m ActivityModel) Init() tea.Cmd {
	return loadActivityCmd(m.db, "")
}

func (m ActivityModel) Update(msg tea.Msg) (ActivityModel, tea.Cmd) {
	switch msg := msg.(type) {
	case activityDataMsg:
		m.loading = false
		m.err = msg.err
		if msg.err == nil {
			if m.autoScroll {
				m.rows = msg.rows
				m.cursor = 0
			} else {
				added := countNewRows(m.rows, msg.rows)
				m.rows = msg.rows
				m.newCount += added
			}
		}

	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height

	case tea.KeyMsg:
		switch msg.String() {
		case "f":
			m.filter = (m.filter + 1) % 5
			m.loading = true
			filter := filterEventTypes[m.filter]
			if m.filter == filterSystem {
				filter = activityFilterSystemActor
			}
			return m, loadActivityCmd(m.db, filter)
		case "up", "k":
			if m.cursor > 0 {
				m.cursor--
				m.autoScroll = false
			}
		case "down", "j":
			if m.cursor < len(m.rows)-1 {
				m.cursor++
			}
			if m.cursor == 0 {
				m.autoScroll = true
				m.newCount = 0
			}
		case "g", "home":
			m.cursor = 0
			m.autoScroll = true
			m.newCount = 0
		case "G", "end":
			m.cursor = len(m.rows) - 1
			m.autoScroll = false
		case " ":
			m.cursor = 0
			m.autoScroll = true
			m.newCount = 0
		case "r":
			return m, m.refreshCmd()
		}
	}
	return m, nil
}

func (m ActivityModel) Refresh() tea.Cmd { return m.refreshCmd() }

func (m ActivityModel) refreshCmd() tea.Cmd {
	filter := filterEventTypes[m.filter]
	if m.filter == filterSystem {
		filter = activityFilterSystemActor
	}
	return loadActivityCmd(m.db, filter)
}

func (m ActivityModel) View() string {
	if m.loading {
		return MutedStyle.Render("  Loading activity...")
	}
	if m.err != nil {
		return viewCenterErr(m.err, m.width, m.height)
	}

	var sb strings.Builder

	// Header with filter indicator
	filterLabel := filterLabels[m.filter]
	header := renderSectionDivider("Recent activity", m.width-20)
	filterStr := MutedStyle.Render("filter: ") + SuccessStyle.Render(filterLabel)
	sb.WriteString(lipgloss.JoinHorizontal(lipgloss.Left, header, "  ", filterStr))
	sb.WriteString("\n")

	// New events indicator
	if m.newCount > 0 && !m.autoScroll {
		indicator := WarningStyle.Render(fmt.Sprintf("↑ %d new events — press space to jump to top", m.newCount))
		sb.WriteString(indicator)
		sb.WriteString("\n")
	}

	// Calculate visible rows
	visibleH := max(m.height-4, 1)

	start := m.cursor
	end := clamp(start+visibleH, 0, len(m.rows))
	visible := m.rows[start:end]

	for _, row := range visible {
		sb.WriteString(renderActivityRow(row, m.width))
		sb.WriteString("\n")
	}

	hints := MutedStyle.Render("↑/↓ scroll   f filter   space jump to top")
	sb.WriteString(hints)

	return sb.String()
}

func renderActivityRow(r ActivityRow, width int) string {
	age := timeAgo(r.CreatedAt)
	eventDisplay := formatEventType(r.EventType, r.Payload)
	actor := r.ActorEmail

	// Color event
	var eventStyled string
	switch r.EventType {
	case "asset_created", "asset_version_uploaded":
		eventStyled = SuccessStyle.Render(eventDisplay)
	case "asset_deleted", "asset_version_deleted":
		eventStyled = ErrorStyle.Render(eventDisplay)
	case "asset_version_restored":
		eventStyled = WarningStyle.Render(eventDisplay)
	default:
		eventStyled = eventDisplay
	}

	// Color actor
	var actorStyled string
	if r.ActorType == "system" {
		actorStyled = ItalicMutedStyle.Render("system")
	} else {
		actorStyled = actor
	}

	ageStr := MutedStyle.Width(12).Render(truncate(age, 12))
	actorStr := lipgloss.NewStyle().Width(28).Render(truncate(actorStyled, 28))
	eventStr := lipgloss.NewStyle().Width(22).Render(truncate(eventStyled, 22))
	assetStr := lipgloss.NewStyle().Width(clamp(width-90, 10, 30)).Render(truncate(r.AssetName, 30))
	wsStr := MutedStyle.Render(truncate(r.WorkspaceName, 20))

	return ageStr + "  " + actorStr + "  " + eventStr + "  " + assetStr + "  " + wsStr
}

func formatEventType(evType, payload string) string {
	switch evType {
	case "asset_created":
		return "uploaded"
	case "asset_renamed":
		return "renamed"
	case "asset_moved":
		return "moved"
	case "asset_tagged":
		tag := extractPayloadField(payload, "tag")
		if tag != "" {
			return "tagged [" + tag + "]"
		}
		return "tagged"
	case "asset_untagged":
		tag := extractPayloadField(payload, "tag")
		if tag != "" {
			return "untagged [" + tag + "]"
		}
		return "untagged"
	case "asset_field_set":
		field := extractPayloadField(payload, "field")
		if field != "" {
			return "set [" + field + "]"
		}
		return "field set"
	case "asset_version_uploaded":
		n := extractPayloadField(payload, "version")
		if n != "" {
			return "uploaded v" + n
		}
		return "new version"
	case "asset_version_restored":
		n := extractPayloadField(payload, "version")
		if n != "" {
			return "restored v" + n
		}
		return "restored"
	case "asset_deleted":
		return "deleted"
	case "asset_shared":
		return "shared"
	default:
		return evType
	}
}

func extractPayloadField(payload, field string) string {
	// Simple JSON field extractor without importing encoding/json for performance
	key := `"` + field + `"`
	_, after, ok := strings.Cut(payload, key)
	if !ok {
		return ""
	}
	rest := after
	colonIdx := strings.Index(rest, ":")
	if colonIdx < 0 {
		return ""
	}
	rest = strings.TrimSpace(rest[colonIdx+1:])
	if len(rest) == 0 {
		return ""
	}
	if rest[0] == '"' {
		end := strings.Index(rest[1:], `"`)
		if end < 0 {
			return ""
		}
		return rest[1 : end+1]
	}
	// Number or unquoted
	end := strings.IndexAny(rest, ",}")
	if end < 0 {
		end = len(rest)
	}
	return strings.TrimSpace(rest[:end])
}

func countNewRows(old, newRow []ActivityRow) int {
	if len(old) == 0 || len(newRow) == 0 {
		return 0
	}
	firstOld := old[0].CreatedAt
	count := 0
	for _, r := range newRow {
		if r.CreatedAt.After(firstOld) {
			count++
		}
	}
	return count
}

// ─── async loader ────────────────────────────────────────────

func loadActivityCmd(db *sql.DB, filter string) tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		// Special case: system filter means actor_type = 'system'
		actualFilter := filter
		if filter == activityFilterSystemActor {
			actualFilter = ""
		}
		rows, err := QueryRecentActivity(ctx, db, activityLimit, actualFilter)
		if err != nil {
			return activityDataMsg{err: err}
		}
		// Apply system filter in-memory
		if filter == activityFilterSystemActor {
			var filtered []ActivityRow
			for _, r := range rows {
				if r.ActorType == "system" {
					filtered = append(filtered, r)
				}
			}
			rows = filtered
		}
		return activityDataMsg{rows: rows}
	}
}

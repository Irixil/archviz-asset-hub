package admin

import (
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
)

func sampleUserRow() UserRow {
	t := time.Now()
	return UserRow{
		ID:            "1",
		Email:         "user@example.com",
		WorkspaceName: "test-ws",
		Role:          "owner",
		AssetCount:    42,
		LastUpload:    &t,
		CreatedAt:     time.Now(),
	}
}

// TestUsersModel_RebuildTable_DoesNotPanic guards against the regression where
// rebuildCols/rebuildTable used value receivers, leaving m.table.cols empty.
// Calling View() after loading data would panic in table.renderRow.
func TestUsersModel_RebuildTable_DoesNotPanic(t *testing.T) {
	for _, tt := range []struct {
		name  string
		width int
	}{
		{"wide", 120},
		{"narrow", 80},
		{"zero-width", 0},
	} {
		t.Run(tt.name, func(t *testing.T) {
			// nil DB is safe: loadCmd is never called in this test.
			m := NewUsersModel(nil)
			m, _ = m.Update(tea.WindowSizeMsg{Width: tt.width, Height: 30})
			m, _ = m.Update(usersDataMsg{
				rows:  []UserRow{sampleUserRow()},
				total: 1,
			})
			// Panics if rebuildCols has a value receiver (cols stay empty).
			out := m.View()
			if out == "" {
				t.Error("View() returned empty string")
			}
		})
	}
}

// Package main is the admin CLI entrypoint.
package main

import (
	"flag"
	"fmt"
	"os"

	"damask/server/internal/admin"

	tea "github.com/charmbracelet/bubbletea"
)

func main() {
	var (
		dbPath     string
		refreshSec int
	)

	flag.StringVar(&dbPath, "db", "", "path to damask.db (default: $DB_PATH env var, then ./damask.db)")
	flag.IntVar(&refreshSec, "refresh", 30, "auto-refresh interval in seconds")
	flag.Parse()

	// Resolve DB path: flag → env var → fallback
	if dbPath == "" {
		if v := os.Getenv("DB_PATH"); v != "" {
			dbPath = v
		} else {
			dbPath = "./damask.db"
		}
	}

	db, err := admin.OpenReadOnly(dbPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "damask-admin: database not found or locked: %v\n", err)
		os.Exit(1)
	}
	defer db.Close()

	// Warn on narrow terminal
	if w, _, e := admin.TermSize(); e == nil && w > 0 && w < 80 {
		fmt.Fprintf(os.Stderr, "damask-admin: terminal width %d < 80; some layout may be clipped\n", w)
	}

	m := admin.NewRootModel(db, dbPath, refreshSec)

	p := tea.NewProgram(m, tea.WithAltScreen())
	if _, err = p.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "damask-admin: %v\n", err)
		os.Exit(1) //nolint: gocritic // Defered db.Close() is not needed on exit.
	}
}

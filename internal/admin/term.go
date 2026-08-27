package admin

import (
	"github.com/charmbracelet/x/term"
)

func TermSize() (int, int, error) {
	return term.GetSize(1) // stdout
}

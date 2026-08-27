package mailserver

import (
	"context"
	"log/slog"

	"github.com/DusanKasan/parsemail"
)

type Hook struct {
	Address     string `yaml:"address"`
	Name        string `yaml:"name"`
	WorkspaceID string `yaml:"workspace_id"`
}

func NewHook(emailAddress, name, workspaceID string) *Hook {
	return &Hook{
		Address:     emailAddress,
		Name:        name,
		WorkspaceID: workspaceID,
	}
}

func (h Hook) Trigger(ctx context.Context, from string, email parsemail.Email) error {
	slog.InfoContext(ctx, "trigger from email", "from", from, "subject", email.Subject)

	return nil
}

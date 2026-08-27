package mailserver

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"os"
	"strings"

	dbgen "damask/server/internal/db/gen"
	"damask/server/internal/ingress"
	"damask/server/internal/queue"
	"damask/server/internal/slug"

	"github.com/DusanKasan/parsemail"
	"github.com/emersion/go-smtp"
	"github.com/google/uuid"
)

var errSourceSkipped = errors.New("ingress source skipped")

// A Session is returned after successful login.
type Session struct {
	hooks   []Hook
	from    string
	to      string
	queries *dbgen.Queries
	queue   queue.JobQueue
}

// AuthPlain implements authentication using SASL PLAIN.
func (s *Session) AuthPlain(_, _ string) error {
	return errors.New("invalid username or password")
}

func (s *Session) Mail(from string, _ *smtp.MailOptions) error {
	slog.Debug("mail from", "from", from)
	s.from = from
	return nil
}

func (s *Session) Rcpt(to string, _ *smtp.RcptOptions) error {
	s.to = to
	return nil
}

func (s *Session) Data(r io.Reader) error {
	email, err := parsemail.Parse(r)
	if err != nil {
		return err
	}

	for _, h := range s.hooks {
		if h.Address == s.to {
			if err = h.Trigger(context.Background(), s.from, email); err != nil {
				return err
			}
		}
	}

	if s.queries == nil || s.queue == nil {
		return nil
	}

	parts := strings.Split(s.to, "@")
	//nolint:mnd // name@host is the expected format
	if len(parts) != 2 {
		return fmt.Errorf("invalid recipient address: %s", s.to)
	}

	ctx := context.Background()
	token, tag := slug.ParseSubaddress(parts[0])

	src, err := s.resolveIngressSource(ctx, token)
	if err != nil || src == nil {
		return nil //nolint:nilerr // skip processing if source is not found or skipped
	}

	overrideFolderID := s.resolveFolderOverride(ctx, *src, tag)

	for _, att := range email.Attachments {
		if err = s.enqueueAttachment(ctx, *src, att, overrideFolderID); err != nil {
			slog.ErrorContext(ctx, "mailserver: enqueue attachment",
				"filename", att.Filename, "source_id", src.ID, "error", err)
		}
	}
	return nil
}

func (s *Session) resolveIngressSource(ctx context.Context, token string) (*dbgen.IngressSource, error) {
	src, err := s.queries.GetIngressSourceByPublicToken(ctx, token)
	if err != nil {
		if !errors.Is(err, sql.ErrNoRows) {
			slog.ErrorContext(ctx, "mailserver: lookup source token",
				"token_prefix", safePrefix(token), "error", err)
		}
		return nil, errSourceSkipped
	}
	if src.Enabled == 0 {
		slog.DebugContext(ctx, "mailserver: ignore disabled source", "token", token)
		return nil, errSourceSkipped
	}
	return &src, nil
}

func (s *Session) resolveFolderOverride(ctx context.Context, src dbgen.IngressSource, tag string) string {
	if tag == "" {
		return ""
	}
	folder, err := s.queries.GetFolderBySlug(ctx, dbgen.GetFolderBySlugParams{
		WorkspaceID: src.WorkspaceID,
		Slug:        &tag,
	})
	if err != nil {
		slog.WarnContext(ctx, "mailserver: no folder for tag (falling back to default)",
			"tag", tag, "workspace_id", src.WorkspaceID)
		return ""
	}
	return folder.ID
}

func (s *Session) enqueueAttachment(
	ctx context.Context,
	src dbgen.IngressSource,
	att parsemail.Attachment,
	overrideFolderID string,
) error {
	tmp, err := os.CreateTemp("", "email-ingest-*")
	if err != nil {
		return fmt.Errorf("create temp: %w", err)
	}
	tmpPath := tmp.Name()
	if _, err = io.Copy(tmp, att.Data); err != nil {
		_ = tmp.Close()
		_ = os.Remove(tmpPath)
		return fmt.Errorf("write temp: %w", err)
	}
	_ = tmp.Close()

	filename := att.Filename
	if filename == "" {
		filename = uuid.NewString()
	}

	entryID := uuid.NewString()
	entry, err := s.queries.InsertIngressLogEntry(ctx, dbgen.InsertIngressLogEntryParams{
		ID:       entryID,
		SourceID: src.ID,
		RemoteID: tmpPath,
		Filename: filename,
	})
	if errors.Is(err, sql.ErrNoRows) { // INSERT OR IGNORE: duplicate
		_ = os.Remove(tmpPath)
		return nil
	}
	if err != nil {
		_ = os.Remove(tmpPath)
		return fmt.Errorf("insert log entry: %w", err)
	}

	payload, _ := json.Marshal(ingress.FetchJobPayload{
		SourceID:         src.ID,
		WorkspaceID:      src.WorkspaceID,
		LogEntryID:       entry.ID,
		RemoteID:         tmpPath,
		Filename:         filename,
		TmpPath:          tmpPath,
		OverrideFolderID: overrideFolderID,
	})

	if _, err = s.queue.Enqueue(ctx, src.WorkspaceID, queue.JobTypeIngestFetch, string(payload)); err != nil {
		_ = os.Remove(tmpPath)
		return fmt.Errorf("enqueue: %w", err)
	}
	return nil
}

func safePrefix(s string) string {
	if len(s) <= 8 { //nolint:mnd // 8 is an arbitrary prefix length that balances utility and safety
		return "***"
	}
	return s[:8] + "..."
}

func (s *Session) Reset() {}

func (s *Session) Logout() error {
	return nil
}

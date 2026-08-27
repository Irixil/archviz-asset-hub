package mail

import (
	"bytes"
	"context"
	"embed"
	"fmt"
	"html/template"
	"log/slog"
	"strings"

	"damask/server/internal/telemetry"

	gomail "github.com/wneessen/go-mail"
	"go.opentelemetry.io/otel/codes"
)

//go:embed templates/*.html
var templatesFS embed.FS

type Config struct {
	Sender   string
	Host     string
	Port     int
	User     string
	Password string
	BaseURL  string
}

type Mailer interface {
	SendInvite(ctx context.Context, workspaceID, to, role, token string) error
	SendWelcome(ctx context.Context, to, username, workspaceID string) error
	SendInviteAccepted(ctx context.Context, workspaceID, to, newMemberName, newMemberEmail, role string) error
	SendIngressSourceAdded(ctx context.Context, to, sourceName, workspaceID string) error
	SendIngressSourceFailed(ctx context.Context, to, sourceName, errMsg, workspaceID string) error
	SendIngressSourceDisabled(ctx context.Context, to, sourceName, errMsg, workspaceID string) error
	SendCommentPosted(ctx context.Context, workspaceID, assetID, to, authorName, shareLabel, commentBody string) error
	SendPasswordReset(ctx context.Context, to, token string) error
	SendEmailChangeConfirmation(ctx context.Context, to, newEmail, token string) error
	SendWorkflowRunFailed(ctx context.Context, to, workflowName, errMsg, workspaceID string) error
}

func NewMailer(config *Config) Mailer {
	mailer := &MailerImpl{
		config:  config,
		BaseURL: config.BaseURL,
	}
	if len(config.Host) > 0 {
		client, err := gomail.NewClient(
			config.Host,
			gomail.WithPort(config.Port),
			gomail.WithSMTPAuth(gomail.SMTPAuthAutoDiscover),
			gomail.WithTLSPortPolicy(gomail.TLSOpportunistic),
			gomail.WithUsername(config.User),
			gomail.WithPassword(config.Password),
		)
		if err != nil {
			slog.Error(
				"mailer: failed to create new mail delivery client",
				"error", err,
				"host", config.Host,
				"port", config.Port,
			)
			panic("unrecoverable error: mailer config is not usable")
		}
		mailer.client = client
	}
	return mailer
}

type MailerImpl struct {
	config  *Config
	client  *gomail.Client
	BaseURL string
}

// renderTemplate parses base.html + the named child template into an isolated
// set and executes it. Each call gets its own set so that block definitions
// from different child templates never collide with each other.
func (m *MailerImpl) renderTemplate(name string, data any) (string, error) {
	tmpl, err := template.ParseFS(templatesFS, "templates/base.html", "templates/"+name+".html")
	if err != nil {
		return "", fmt.Errorf("mail: parse template %q: %w", name, err)
	}
	var buf bytes.Buffer
	if err = tmpl.ExecuteTemplate(&buf, name+".html", data); err != nil {
		return "", fmt.Errorf("mail: execute template %q: %w", name, err)
	}
	out := buf.String()
	if strings.Contains(out, "{{") {
		return "", fmt.Errorf("mail: template %q has unresolved actions in output", name)
	}
	return out, nil
}

func (m *MailerImpl) deliver(ctx context.Context, to, subject, htmlBody string) error {
	msg, err := m.prepare(to, subject, htmlBody)
	if err != nil {
		return err
	}
	return m.send(ctx, msg)
}

func (m *MailerImpl) prepare(to, subject, body string) (message *gomail.Msg, err error) {
	message = gomail.NewMsg()
	if err = message.From(m.config.Sender); err != nil {
		slog.Error("mailer: failed to set [from] address", "error", err)
		return nil, err
	}
	if err = message.To(to); err != nil {
		slog.Error("mailer: failed to set [to] address", "error", err)
		return nil, err
	}
	message.Subject(subject)
	message.SetBodyString(gomail.TypeTextHTML, body)
	return message, nil
}

func (m *MailerImpl) send(ctx context.Context, message *gomail.Msg) (err error) {
	ctx, span := telemetry.StartSpan(ctx, "mail.send")
	defer telemetry.EndSpan(span, err)

	if m.client != nil {
		if err = m.client.DialAndSendWithContext(ctx, message); err != nil {
			slog.ErrorContext(ctx, "mailer: failed to deliver mail", "error", err)
			return err
		}
	} else {
		err = message.WriteToSendmailWithContext(ctx, "sendmail")
		if err != nil {
			slog.ErrorContext(ctx, "mailer: failed to deliver mail with local sendmail", "error", err)
		}
	}

	span.SetStatus(codes.Ok, "mail sent")
	return err
}

func (m *MailerImpl) SendInvite(ctx context.Context, workspaceID, to, role, token string) error {
	base := m.newBaseData(workspaceID, "INVITATION", "")
	base.Preheader = "You've been invited to join a workspace on Damask."
	inviteURL := m.BaseURL + "/invite?token=" + token
	if workspaceID != "" {
		inviteURL += "&ws=" + workspaceID
	}
	data := InviteData{
		BaseData:  base,
		Role:      role,
		InviteURL: inviteURL,
	}
	html, err := m.renderTemplate("invite", data)
	if err != nil {
		return err
	}
	return m.deliver(ctx, to, "You've been invited to Damask", html)
}

func (m *MailerImpl) SendWelcome(ctx context.Context, to, username, workspaceID string) error {
	base := m.newBaseData(workspaceID, "WELCOME", "")
	base.Preheader = "Your Damask workspace is ready. Let's get started."
	data := WelcomeData{
		BaseData:   base,
		Username:   username,
		LibraryURL: m.BaseURL + "/library?ws=" + workspaceID,
	}
	html, err := m.renderTemplate("welcome", data)
	if err != nil {
		return err
	}
	return m.deliver(ctx, to, "Welcome to Damask, "+username+"!", html)
}

func (m *MailerImpl) SendInviteAccepted(
	ctx context.Context,
	workspaceID, to, newMemberName, newMemberEmail, role string,
) error {
	base := m.newBaseData(workspaceID, "A NEW MEMBER", "")
	base.Preheader = newMemberName + " accepted your invitation to the studio."
	manageMembersURL := m.BaseURL + "/settings/members"
	if workspaceID != "" {
		manageMembersURL += "?ws=" + workspaceID
	}
	data := InviteAcceptedData{
		BaseData:         base,
		NewMemberName:    newMemberName,
		NewMemberEmail:   newMemberEmail,
		NewMemberInitial: InitialFromName(newMemberName),
		Role:             role,
		RoleDisplay:      strings.ToUpper(role),
		RoleDescription:  RoleDescription(role),
		ManageMembersURL: manageMembersURL,
	}
	html, err := m.renderTemplate("invite_accepted", data)
	if err != nil {
		return err
	}
	return m.deliver(ctx, to, newMemberName+" accepted your invitation", html)
}

func (m *MailerImpl) SendIngressSourceAdded(ctx context.Context, to, sourceName, workspaceID string) error {
	base := m.newBaseData(workspaceID, "INGRESS", "")
	base.Preheader = "Your ingress source “" + sourceName + "” is now active."
	data := IngressSourceAddedData{
		BaseData:   base,
		SourceName: sourceName,
		IngressURL: m.BaseURL + "/settings/ingress?ws=" + workspaceID,
	}
	html, err := m.renderTemplate("ingress_added", data)
	if err != nil {
		return err
	}
	return m.deliver(ctx, to, "Ingress source connected: "+sourceName, html)
}

func (m *MailerImpl) SendIngressSourceFailed(ctx context.Context, to, sourceName, errMsg, workspaceID string) error {
	base := m.newBaseData(workspaceID, "INGRESS ERROR", "")
	base.Preheader = "Your ingress source “" + sourceName + "” has encountered an error."
	data := IngressSourceFailedData{
		BaseData:   base,
		SourceName: sourceName,
		ErrMsg:     errMsg,
		IngressURL: m.BaseURL + "/settings/ingress?ws=" + workspaceID,
	}
	html, err := m.renderTemplate("ingress_failed", data)
	if err != nil {
		return err
	}
	return m.deliver(ctx, to, "Ingress source error: "+sourceName, html)
}

func (m *MailerImpl) SendIngressSourceDisabled(ctx context.Context, to, sourceName, errMsg, workspaceID string) error {
	base := m.newBaseData(workspaceID, "INGRESS DISABLED", "")
	base.Preheader = "Your ingress source “" + sourceName + "” has been disabled."
	data := IngressSourceDisabledData{
		BaseData:   base,
		SourceName: sourceName,
		ErrMsg:     errMsg,
		IngressURL: m.BaseURL + "/settings/ingress?ws=" + workspaceID,
	}
	html, err := m.renderTemplate("ingress_disabled", data)
	if err != nil {
		return err
	}
	return m.deliver(ctx, to, "Ingress source disabled: "+sourceName, html)
}

func (m *MailerImpl) SendCommentPosted(
	ctx context.Context,
	workspaceID, assetID, to, authorName, shareLabel, commentBody string,
) error {
	base := m.newBaseData(workspaceID, "NEW COMMENT", "")
	base.Preheader = authorName + " commented on \"" + shareLabel + "\"."
	shareURL := m.BaseURL + "/library?ws=" + workspaceID + "&asset=" + assetID
	data := CommentData{
		BaseData:    base,
		AuthorName:  authorName,
		ShareLabel:  shareLabel,
		CommentBody: commentBody,
		ShareURL:    shareURL,
	}
	html, err := m.renderTemplate("comment", data)
	if err != nil {
		return err
	}
	return m.deliver(ctx, to, authorName+" commented on "+shareLabel, html)
}

func (m *MailerImpl) SendPasswordReset(ctx context.Context, to, token string) error {
	base := m.newBaseData("", "ACCOUNT SECURITY", "")
	base.Preheader = "Reset your Damask password - this link expires in 1 hour."
	data := PasswordResetData{
		BaseData: base,
		ResetURL: m.BaseURL + "/reset-password?token=" + token,
	}
	html, err := m.renderTemplate("password_reset", data)
	if err != nil {
		return err
	}
	return m.deliver(ctx, to, "Reset your Damask password", html)
}

func (m *MailerImpl) SendEmailChangeConfirmation(ctx context.Context, to, newEmail, token string) error {
	base := m.newBaseData("", "EMAIL CHANGE", "")
	base.Preheader = "Confirm your new email address for Damask."
	data := EmailChangeData{
		BaseData:   base,
		NewEmail:   newEmail,
		ConfirmURL: m.BaseURL + "/confirm-email?token=" + token,
	}
	html, err := m.renderTemplate("email_change", data)
	if err != nil {
		return err
	}
	return m.deliver(ctx, to, "Confirm your new email address", html)
}

func (m *MailerImpl) SendWorkflowRunFailed(ctx context.Context, to, workflowName, errMsg, workspaceID string) error {
	base := m.newBaseData(workspaceID, "WORKFLOW ERROR", "")
	base.Preheader = "Workflow “" + workflowName + "” failed to complete."
	data := WorkflowFailedData{
		BaseData:     base,
		WorkflowName: workflowName,
		ErrMsg:       errMsg,
		WorkflowsURL: m.BaseURL + "/settings/workflows?ws=" + workspaceID,
	}
	html, err := m.renderTemplate("workflow_failed", data)
	if err != nil {
		return err
	}
	return m.deliver(ctx, to, "Workflow failed: "+workflowName, html)
}

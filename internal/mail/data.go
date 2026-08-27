// Package mail defines mail template data structures.
package mail

import (
	"strings"
	"time"
)

// ─────────────────────────────────────────────────────────────────────────────
// Base data
// ─────────────────────────────────────────────────────────────────────────────

// BaseData is embedded in every email data struct. It carries the shared
// structural fields that base.html depends on.
type BaseData struct {
	// AppName is the product name shown in the header and footer ("Damask").
	AppName string
	// Date is the formatted send date shown in the header subtitle, e.g. "09 MAY".
	Date string
	// EventLabel is a short uppercase label shown alongside Date in the header,
	// e.g. "A NEW MEMBER", "WELCOME", "ACCOUNT SECURITY".
	EventLabel string
	// WorkspaceName is shown above the headline in the body. May be empty —
	// in that case the section is hidden.
	WorkspaceName string
	// Preheader is the invisible inbox-preview text injected at the top of the body.
	Preheader string
	// SettingsURL links to /settings?ws=<id> (or /settings when no workspace ID).
	SettingsURL string
	// WorkspaceURL links to /library?ws=<id> (or /library when no workspace ID).
	WorkspaceURL string
}

// newBaseData constructs a BaseData for a given workspace ID, event label and
// optional workspace name. Pass empty strings for workspaceID / workspaceName
// when they are unavailable at the call site.
func (m *MailerImpl) newBaseData(workspaceID, eventLabel, workspaceName string) BaseData {
	settingsURL := m.BaseURL + "/settings"
	wsURL := m.BaseURL + "/library"
	if workspaceID != "" {
		settingsURL += "?ws=" + workspaceID
		wsURL += "?ws=" + workspaceID
	}
	return BaseData{
		AppName:       "Damask",
		Date:          strings.ToUpper(time.Now().Format("02 Jan")),
		EventLabel:    eventLabel,
		WorkspaceName: workspaceName,
		SettingsURL:   settingsURL,
		WorkspaceURL:  wsURL,
	}
}

// ─────────────────────────────────────────────────────────────────────────────
// Per-email data structs
// ─────────────────────────────────────────────────────────────────────────────

// InviteData is used by SendInvite.
type InviteData struct {
	BaseData

	Role      string // e.g. "Viewer"
	InviteURL string // full URL including token
}

// WelcomeData is used by SendWelcome.
type WelcomeData struct {
	BaseData

	Username   string
	LibraryURL string
}

// InviteAcceptedData is used by SendInviteAccepted.
type InviteAcceptedData struct {
	BaseData

	NewMemberName    string // "Rez Halabi"
	NewMemberEmail   string // "rez@halabi.studio"
	NewMemberInitial string // "R"
	Role             string // "Viewer"
	RoleDisplay      string // "VIEWER" (uppercase, used in the badge)
	RoleDescription  string // "They can browse, comment and download…"
	ManageMembersURL string
}

// IngressSourceAddedData is used by SendIngressSourceAdded.
type IngressSourceAddedData struct {
	BaseData

	SourceName string
	IngressURL string // links to ingress settings for this workspace
}

// IngressSourceFailedData is used by SendIngressSourceFailed.
type IngressSourceFailedData struct {
	BaseData

	SourceName string
	ErrMsg     string
	IngressURL string
}

// IngressSourceDisabledData is used by SendIngressSourceDisabled.
type IngressSourceDisabledData struct {
	BaseData

	SourceName string
	ErrMsg     string
	IngressURL string
}

// CommentData is used by SendCommentPosted.
type CommentData struct {
	BaseData

	AuthorName  string
	ShareLabel  string
	CommentBody string
	ShareURL    string
}

// PasswordResetData is used by SendPasswordReset.
type PasswordResetData struct {
	BaseData

	ResetURL string
}

// EmailChangeData is used by SendEmailChangeConfirmation.
type EmailChangeData struct {
	BaseData

	NewEmail   string
	ConfirmURL string
}

// WorkflowFailedData is used by SendWorkflowRunFailed.
type WorkflowFailedData struct {
	BaseData

	WorkflowName string
	ErrMsg       string
	WorkflowsURL string
}

// ─────────────────────────────────────────────────────────────────────────────
// Helpers
// ─────────────────────────────────────────────────────────────────────────────

// RoleDescription returns a human-readable sentence describing what a member
// with the given role can do in the workspace.
func RoleDescription(role string) string {
	switch strings.ToLower(strings.TrimSpace(role)) {
	case "viewer":
		return "They can browse, comment and download — but not edit or upload files."
	case "editor":
		return "They can upload, edit, and organise files — but cannot manage members or billing."
	case "admin":
		return "They have full access to the workspace, including member management."
	default:
		return "They have access to the workspace."
	}
}

// InitialFromName returns the first Unicode letter of name, uppercased.
// Returns "?" when name is blank.
func InitialFromName(name string) string {
	name = strings.TrimSpace(name)
	if name == "" {
		return "?"
	}
	return strings.ToUpper(string([]rune(name)[0]))
}

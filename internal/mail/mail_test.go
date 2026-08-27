package mail

import (
	"strings"
	"testing"
	"time"
)

// ─────────────────────────────────────────────────────────────────────────────
// Test fixture
// ─────────────────────────────────────────────────────────────────────────────

// testMailer returns a MailerImpl wired to a no-op send function for rendering
// tests. It never dials SMTP.
func testMailer() *MailerImpl {
	return &MailerImpl{
		BaseURL: "https://app.example.com",
	}
}

// assertRender executes renderTemplate and runs a set of basic sanity checks
// that apply to every email. Returns the rendered HTML for further assertions.
func assertRender(t *testing.T, m *MailerImpl, tmplName string, data any) string {
	t.Helper()
	html, err := m.renderTemplate(tmplName, data)
	if err != nil {
		t.Fatalf("renderTemplate(%q) error: %v", tmplName, err)
	}
	if strings.TrimSpace(html) == "" {
		t.Fatalf("renderTemplate(%q) returned empty output", tmplName)
	}
	if strings.Contains(html, "{{") {
		t.Fatalf("renderTemplate(%q) left unresolved template actions in output", tmplName)
	}
	if !strings.Contains(html, "D A M A S K") {
		t.Errorf("renderTemplate(%q): expected Damask wordmark in output", tmplName)
	}
	return html
}

// ─────────────────────────────────────────────────────────────────────────────
// Invite
// ─────────────────────────────────────────────────────────────────────────────

func TestRenderInvite(t *testing.T) {
	m := testMailer()
	base := m.newBaseData("", "INVITATION", "")
	base.Preheader = "You've been invited."
	data := InviteData{
		BaseData:  base,
		Role:      "Editor",
		InviteURL: "https://app.example.com/invite?token=tok123",
	}

	html := assertRender(t, m, "invite", data)

	for _, want := range []string{
		"Editor",
		"tok123",
		"Accept invitation",
		"INVITATION",
	} {
		if !strings.Contains(html, want) {
			t.Errorf("invite: expected %q in output", want)
		}
	}
}

// ─────────────────────────────────────────────────────────────────────────────
// Welcome
// ─────────────────────────────────────────────────────────────────────────────

func TestRenderWelcome(t *testing.T) {
	m := testMailer()
	base := m.newBaseData("ws_abc", "WELCOME", "")
	base.Preheader = "Welcome."
	data := WelcomeData{
		BaseData:   base,
		Username:   "Alice",
		LibraryURL: "https://app.example.com/library?ws=ws_abc",
	}

	html := assertRender(t, m, "welcome", data)

	for _, want := range []string{
		"Alice",
		"Explore your library",
		"ws_abc",
	} {
		if !strings.Contains(html, want) {
			t.Errorf("welcome: expected %q in output", want)
		}
	}
}

// ─────────────────────────────────────────────────────────────────────────────
// InviteAccepted
// ─────────────────────────────────────────────────────────────────────────────

func TestRenderInviteAccepted(t *testing.T) {
	m := testMailer()
	base := m.newBaseData("", "A NEW MEMBER", "Atelier Vento")
	base.Preheader = "Rez accepted your invitation."
	data := InviteAcceptedData{
		BaseData:         base,
		NewMemberName:    "Rez Halabi",
		NewMemberEmail:   "rez@halabi.studio",
		NewMemberInitial: "R",
		Role:             "Viewer",
		RoleDisplay:      "VIEWER",
		RoleDescription:  RoleDescription("Viewer"),
		ManageMembersURL: "https://app.example.com/settings/members",
	}

	html := assertRender(t, m, "invite_accepted", data)

	for _, want := range []string{
		"Rez Halabi",
		"rez@halabi.studio",
		"R",      // initial in avatar
		"VIEWER", // badge
		"Viewer", // role in body text
		"browse, comment and download",
		"Atelier Vento",
		"A NEW MEMBER",
		"Manage members",
		"settings/members",
	} {
		if !strings.Contains(html, want) {
			t.Errorf("invite_accepted: expected %q in output", want)
		}
	}
}

func TestRenderInviteAccepted_NoWorkspaceName(t *testing.T) {
	m := testMailer()
	base := m.newBaseData("", "A NEW MEMBER", "")
	data := InviteAcceptedData{
		BaseData:         base,
		NewMemberName:    "Bob",
		NewMemberEmail:   "bob@example.com",
		NewMemberInitial: "B",
		Role:             "Admin",
		RoleDisplay:      "ADMIN",
		RoleDescription:  RoleDescription("Admin"),
		ManageMembersURL: "https://app.example.com/settings/members",
	}

	html := assertRender(t, m, "invite_accepted", data)
	if strings.Contains(html, `></p>`) {
		t.Errorf("invite_accepted: found empty <p> tag — conditional block may be broken")
	}
}

// ─────────────────────────────────────────────────────────────────────────────
// Ingress
// ─────────────────────────────────────────────────────────────────────────────

func TestRenderIngressAdded(t *testing.T) {
	m := testMailer()
	base := m.newBaseData("ws_xyz", "INGRESS", "")
	base.Preheader = "Source connected."
	data := IngressSourceAddedData{
		BaseData:   base,
		SourceName: "Client FTP",
		IngressURL: "https://app.example.com/settings/ingress?ws=ws_xyz",
	}

	html := assertRender(t, m, "ingress_added", data)

	for _, want := range []string{"Client FTP", "connected", "View ingress sources"} {
		if !strings.Contains(html, want) {
			t.Errorf("ingress_added: expected %q in output", want)
		}
	}
}

func TestRenderIngressFailed(t *testing.T) {
	m := testMailer()
	base := m.newBaseData("ws_xyz", "INGRESS ERROR", "")
	data := IngressSourceFailedData{
		BaseData:   base,
		SourceName: "Client FTP",
		ErrMsg:     "connection refused: dial tcp 10.0.0.1:21",
		IngressURL: "https://app.example.com/settings/ingress?ws=ws_xyz",
	}

	html := assertRender(t, m, "ingress_failed", data)

	for _, want := range []string{"Client FTP", "connection refused", "Review ingress settings"} {
		if !strings.Contains(html, want) {
			t.Errorf("ingress_failed: expected %q in output", want)
		}
	}
}

func TestRenderIngressDisabled(t *testing.T) {
	m := testMailer()
	base := m.newBaseData("ws_xyz", "INGRESS DISABLED", "")
	data := IngressSourceDisabledData{
		BaseData:   base,
		SourceName: "Client FTP",
		ErrMsg:     "auth failed after 5 retries",
		IngressURL: "https://app.example.com/settings/ingress?ws=ws_xyz",
	}

	html := assertRender(t, m, "ingress_disabled", data)

	for _, want := range []string{"Client FTP", "auth failed", "Re-enable source"} {
		if !strings.Contains(html, want) {
			t.Errorf("ingress_disabled: expected %q in output", want)
		}
	}
}

// ─────────────────────────────────────────────────────────────────────────────
// Comment
// ─────────────────────────────────────────────────────────────────────────────

func TestRenderComment(t *testing.T) {
	m := testMailer()
	base := m.newBaseData("", "NEW COMMENT", "")
	base.Preheader = "Alice commented."
	data := CommentData{
		BaseData:    base,
		AuthorName:  "Alice",
		ShareLabel:  "Brand Assets Q2",
		CommentBody: "Love the new colour palette!",
		ShareURL:    "https://app.example.com/shares/abc",
	}

	html := assertRender(t, m, "comment", data)

	for _, want := range []string{
		"Alice",
		"Brand Assets Q2",
		"Love the new colour palette!",
		"View shared asset",
	} {
		if !strings.Contains(html, want) {
			t.Errorf("comment: expected %q in output", want)
		}
	}
}

// ─────────────────────────────────────────────────────────────────────────────
// Password reset
// ─────────────────────────────────────────────────────────────────────────────

func TestRenderPasswordReset(t *testing.T) {
	m := testMailer()
	base := m.newBaseData("", "ACCOUNT SECURITY", "")
	base.Preheader = "Reset your password."
	data := PasswordResetData{
		BaseData: base,
		ResetURL: "https://app.example.com/reset-password?token=reset_tok",
	}

	html := assertRender(t, m, "password_reset", data)

	for _, want := range []string{"reset_tok", "Reset password", "1 hour"} {
		if !strings.Contains(html, want) {
			t.Errorf("password_reset: expected %q in output", want)
		}
	}
}

// ─────────────────────────────────────────────────────────────────────────────
// Email change
// ─────────────────────────────────────────────────────────────────────────────

func TestRenderEmailChange(t *testing.T) {
	m := testMailer()
	base := m.newBaseData("", "EMAIL CHANGE", "")
	base.Preheader = "Confirm your new email."
	data := EmailChangeData{
		BaseData:   base,
		NewEmail:   "new@example.com",
		ConfirmURL: "https://app.example.com/confirm-email?token=conf_tok",
	}

	html := assertRender(t, m, "email_change", data)

	for _, want := range []string{"new@example.com", "conf_tok", "Confirm new address"} {
		if !strings.Contains(html, want) {
			t.Errorf("email_change: expected %q in output", want)
		}
	}
}

// ─────────────────────────────────────────────────────────────────────────────
// Workflow failed
// ─────────────────────────────────────────────────────────────────────────────

func TestRenderWorkflowFailed(t *testing.T) {
	m := testMailer()
	base := m.newBaseData("ws_abc", "WORKFLOW ERROR", "")
	base.Preheader = "Workflow failed."
	data := WorkflowFailedData{
		BaseData:     base,
		WorkflowName: "Auto-tag on upload",
		ErrMsg:       "timeout: step 3 exceeded 30s limit",
		WorkflowsURL: "https://app.example.com/settings/workflows?ws=ws_abc",
	}

	html := assertRender(t, m, "workflow_failed", data)

	for _, want := range []string{
		"Auto-tag on upload",
		"timeout: step 3",
		"View workflows",
	} {
		if !strings.Contains(html, want) {
			t.Errorf("workflow_failed: expected %q in output", want)
		}
	}
}

// ─────────────────────────────────────────────────────────────────────────────
// URL construction via newBaseData
// ─────────────────────────────────────────────────────────────────────────────

func TestNewBaseData_WithWorkspaceID(t *testing.T) {
	m := testMailer()
	bd := m.newBaseData("ws_123", "LABEL", "My Studio")

	if !strings.Contains(bd.SettingsURL, "ws=ws_123") {
		t.Errorf("SettingsURL missing workspace ID: %s", bd.SettingsURL)
	}
	if !strings.Contains(bd.WorkspaceURL, "ws=ws_123") {
		t.Errorf("WorkspaceURL missing workspace ID: %s", bd.WorkspaceURL)
	}
	if bd.WorkspaceName != "My Studio" {
		t.Errorf("WorkspaceName = %q, want %q", bd.WorkspaceName, "My Studio")
	}
	if bd.EventLabel != "LABEL" {
		t.Errorf("EventLabel = %q, want %q", bd.EventLabel, "LABEL")
	}
	wantMonth := strings.ToUpper(time.Now().Format("Jan"))
	if !strings.Contains(bd.Date, wantMonth) {
		t.Errorf("Date = %q, does not contain current month %q", bd.Date, wantMonth)
	}
}

func TestNewBaseData_WithoutWorkspaceID(t *testing.T) {
	m := testMailer()
	bd := m.newBaseData("", "LABEL", "")

	if strings.Contains(bd.SettingsURL, "ws=") {
		t.Errorf("SettingsURL should not contain ws= when workspaceID is empty: %s", bd.SettingsURL)
	}
	if !strings.HasSuffix(bd.SettingsURL, "/settings") {
		t.Errorf("SettingsURL = %q, want suffix /settings", bd.SettingsURL)
	}
}

// ─────────────────────────────────────────────────────────────────────────────
// Helper: RoleDescription
// ─────────────────────────────────────────────────────────────────────────────

func TestRoleDescription(t *testing.T) {
	cases := []struct {
		role    string
		contain string
	}{
		{"viewer", "browse"},
		{"Viewer", "browse"},
		{"VIEWER", "browse"},
		{"editor", "upload"},
		{"Editor", "upload"},
		{"admin", "full access"},
		{"Admin", "full access"},
		{"owner", "access"}, // fallback
		{"", "access"},      // fallback
	}
	for _, c := range cases {
		got := RoleDescription(c.role)
		if !strings.Contains(got, c.contain) {
			t.Errorf("RoleDescription(%q) = %q, want it to contain %q", c.role, got, c.contain)
		}
	}
}

// ─────────────────────────────────────────────────────────────────────────────
// Helper: InitialFromName
// ─────────────────────────────────────────────────────────────────────────────

func TestInitialFromName(t *testing.T) {
	cases := []struct {
		name string
		want string
	}{
		{"Rez Halabi", "R"},
		{"alice", "A"},
		{"  bob  ", "B"},
		{"", "?"},
		{"   ", "?"},
		{"ñoño", "Ñ"},
	}
	for _, c := range cases {
		got := InitialFromName(c.name)
		if got != c.want {
			t.Errorf("InitialFromName(%q) = %q, want %q", c.name, got, c.want)
		}
	}
}

// ─────────────────────────────────────────────────────────────────────────────
// Template rendering is deterministic across calls
// ─────────────────────────────────────────────────────────────────────────────

func TestRenderIsDeterministic(t *testing.T) {
	m := testMailer()
	base := m.newBaseData("", "ACCOUNT SECURITY", "")
	data := PasswordResetData{
		BaseData: base,
		ResetURL: "https://app.example.com/reset-password?token=abc",
	}
	h1, err := m.renderTemplate("password_reset", data)
	if err != nil {
		t.Fatal(err)
	}
	h2, err := m.renderTemplate("password_reset", data)
	if err != nil {
		t.Fatal(err)
	}
	if h1 != h2 {
		t.Error("expected renderTemplate to produce identical output on consecutive calls")
	}
}

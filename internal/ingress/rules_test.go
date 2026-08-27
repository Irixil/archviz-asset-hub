package ingress

import (
	"testing"

	dbgen "damask/server/internal/db/gen"
)

func rule(field, operator, value, action string) dbgen.IngressRule {
	return dbgen.IngressRule{
		Field:    field,
		Operator: operator,
		Value:    value,
		Action:   action,
	}
}

// --- EvaluateRules: default behaviour

func TestEvaluateRules_NoRules_AllowsEverything(t *testing.T) {
	t.Parallel()
	res := EvaluateRules(nil, ItemMeta{Filename: "photo.jpg", MimeType: "image/jpeg", Size: 100})
	if !res.Allow {
		t.Fatal("expected Allow=true with no rules")
	}
}

func TestEvaluateRules_NoDenyFired_AllowsByDefault(t *testing.T) {
	t.Parallel()
	rules := []dbgen.IngressRule{
		rule("filename", "contains", "invoice", "deny"),
	}
	res := EvaluateRules(rules, ItemMeta{Filename: "photo.jpg", MimeType: "image/jpeg", Size: 100})
	if !res.Allow {
		t.Fatal("expected Allow=true when no deny rule matches")
	}
}

// --- deny action

func TestEvaluateRules_DenyByFilename(t *testing.T) {
	t.Parallel()
	rules := []dbgen.IngressRule{
		rule("filename", "contains", "invoice", "deny"),
	}
	res := EvaluateRules(rules, ItemMeta{Filename: "invoice-2024.pdf"})
	if res.Allow {
		t.Fatal("expected Allow=false")
	}
}

func TestEvaluateRules_DenyByMimeType(t *testing.T) {
	t.Parallel()
	rules := []dbgen.IngressRule{
		rule("mime_type", "equals", "application/pdf", "deny"),
	}
	res := EvaluateRules(rules, ItemMeta{Filename: "file.pdf", MimeType: "application/pdf"})
	if res.Allow {
		t.Fatal("expected deny for PDF mime type")
	}
}

func TestEvaluateRules_DenyBySizeGt(t *testing.T) {
	t.Parallel()
	rules := []dbgen.IngressRule{
		rule("size", "gt", "1000", "deny"),
	}
	res := EvaluateRules(rules, ItemMeta{Size: 2000})
	if res.Allow {
		t.Fatal("expected deny for size > 1000")
	}
}

func TestEvaluateRules_DenyBySizeLt(t *testing.T) {
	t.Parallel()
	rules := []dbgen.IngressRule{
		rule("size", "lt", "100", "deny"),
	}
	res := EvaluateRules(rules, ItemMeta{Size: 50})
	if res.Allow {
		t.Fatal("expected deny for size < 100")
	}
}

func TestEvaluateRules_SizeGtBoundary_NotMatching(t *testing.T) {
	t.Parallel()
	rules := []dbgen.IngressRule{
		rule("size", "gt", "100", "deny"),
	}
	// Equal to boundary: not strictly greater than
	res := EvaluateRules(rules, ItemMeta{Size: 100})
	if !res.Allow {
		t.Fatal("size equal to boundary should not match 'gt'")
	}
}

func TestEvaluateRules_SizeInvalidValue_SkipsRule(t *testing.T) {
	t.Parallel()
	rules := []dbgen.IngressRule{
		rule("size", "gt", "not-a-number", "deny"),
	}
	// Invalid value → rule is skipped → default allow
	res := EvaluateRules(rules, ItemMeta{Size: 9999})
	if !res.Allow {
		t.Fatal("invalid size value should skip the rule")
	}
}

// --- allow action (explicit)

func TestEvaluateRules_ExplicitAllowDoesNotDeny(t *testing.T) {
	t.Parallel()
	rules := []dbgen.IngressRule{
		rule("filename", "contains", "photo", "allow"),
	}
	res := EvaluateRules(rules, ItemMeta{Filename: "photo.jpg"})
	if !res.Allow {
		t.Fatal("explicit allow should keep Allow=true")
	}
}

// --- set_project / set_folder routing

// Note: in the current implementation, the Value field of a rule serves double
// duty — it is used BOTH as the match operand AND as the destination ID for
// set_project / set_folder actions. Tests below use the same value for both.

func TestEvaluateRules_SetProjectOverride(t *testing.T) {
	t.Parallel()
	// Value = "image/" is both the match value (starts_with) and the project ID.
	rules := []dbgen.IngressRule{
		{Field: "mime_type", Operator: "starts_with", Value: "image/", Action: "set_project"},
	}
	res := EvaluateRules(rules, ItemMeta{MimeType: "image/jpeg"})
	if !res.Allow {
		t.Fatal("set_project should not deny")
	}
	if res.ProjectID == nil || *res.ProjectID != "image/" {
		t.Fatalf("expected ProjectID=%q, got %v", "image/", res.ProjectID)
	}
}

func TestEvaluateRules_SetFolderOverride(t *testing.T) {
	t.Parallel()
	// Value = "pdf" is both the match operand (ends_with) and the folder ID.
	rules := []dbgen.IngressRule{
		{Field: "filename", Operator: "ends_with", Value: "pdf", Action: "set_folder"},
	}
	res := EvaluateRules(rules, ItemMeta{Filename: "report.pdf"})
	if !res.Allow {
		t.Fatal("set_folder should not deny")
	}
	if res.FolderID == nil || *res.FolderID != "pdf" {
		t.Fatalf("expected FolderID=%q, got %v", "pdf", res.FolderID)
	}
}

func TestEvaluateRules_LastSetFolderWins(t *testing.T) {
	t.Parallel()
	// Both rules match "report.pdf" via contains; last set_folder wins.
	rules := []dbgen.IngressRule{
		{Field: "filename", Operator: "contains", Value: "report", Action: "set_folder"},
		{Field: "filename", Operator: "contains", Value: "report", Action: "set_folder"},
	}
	// Override values post-construction to simulate two different folder IDs
	// while still matching the filename.
	rules[0].Value = "folder-a"
	rules[1].Value = "folder-a" // both must match "report.pdf" via contains "folder-a"? No —
	// Actually both use "contains" on filename. We need both values to match "report.pdf".
	// Use a value that is a substring of the filename.
	rules[0] = dbgen.IngressRule{Field: "filename", Operator: "contains", Value: "report", Action: "set_folder"}
	rules[1] = dbgen.IngressRule{Field: "filename", Operator: "contains", Value: ".pdf", Action: "set_folder"}
	res := EvaluateRules(rules, ItemMeta{Filename: "report.pdf"})
	// Both match; last set_folder value (.pdf) wins.
	if res.FolderID == nil || *res.FolderID != ".pdf" {
		t.Fatalf("expected last set_folder to win with value '.pdf', got %v", res.FolderID)
	}
}

// --- deny short-circuits everything

func TestEvaluateRules_DenyShortCircuits(t *testing.T) {
	t.Parallel()
	rules := []dbgen.IngressRule{
		{Field: "filename", Operator: "contains", Value: "bad", Action: "deny"},
		{Field: "filename", Operator: "contains", Value: "bad", Action: "set_folder"},
	}
	rules[1].Value = "some-folder"
	res := EvaluateRules(rules, ItemMeta{Filename: "bad-file.jpg"})
	if res.Allow {
		t.Fatal("deny should short-circuit; expected Allow=false")
	}
	if res.FolderID != nil {
		t.Fatal("deny should short-circuit before set_folder")
	}
}

// --- operator coverage: equals (case-insensitive)

func TestMatchesRule_EqualsIsCaseInsensitive(t *testing.T) {
	t.Parallel()
	rules := []dbgen.IngressRule{
		{Field: "mime_type", Operator: "equals", Value: "IMAGE/JPEG", Action: "deny"},
	}
	res := EvaluateRules(rules, ItemMeta{MimeType: "image/jpeg"})
	if res.Allow {
		t.Fatal("equals should be case-insensitive")
	}
}

func TestMatchesRule_StartsWithIsCaseInsensitive(t *testing.T) {
	t.Parallel()
	rules := []dbgen.IngressRule{
		{Field: "mime_type", Operator: "starts_with", Value: "IMAGE/", Action: "deny"},
	}
	res := EvaluateRules(rules, ItemMeta{MimeType: "image/png"})
	if res.Allow {
		t.Fatal("starts_with should be case-insensitive")
	}
}

func TestMatchesRule_EndsWithIsCaseInsensitive(t *testing.T) {
	t.Parallel()
	rules := []dbgen.IngressRule{
		{Field: "filename", Operator: "ends_with", Value: ".PDF", Action: "deny"},
	}
	res := EvaluateRules(rules, ItemMeta{Filename: "document.pdf"})
	if res.Allow {
		t.Fatal("ends_with should be case-insensitive")
	}
}

func TestMatchesRule_ContainsIsCaseInsensitive(t *testing.T) {
	t.Parallel()
	rules := []dbgen.IngressRule{
		{Field: "filename", Operator: "contains", Value: "INVOICE", Action: "deny"},
	}
	res := EvaluateRules(rules, ItemMeta{Filename: "Invoice-2024.pdf"})
	if res.Allow {
		t.Fatal("contains should be case-insensitive")
	}
}

// --- unknown field / operator

func TestMatchesRule_UnknownField_SkipsRule(t *testing.T) {
	t.Parallel()
	rules := []dbgen.IngressRule{
		{Field: "sender", Operator: "equals", Value: "foo@bar.com", Action: "deny"},
	}
	res := EvaluateRules(rules, ItemMeta{Filename: "x.jpg"})
	if !res.Allow {
		t.Fatal("unknown field should be skipped (not deny)")
	}
}

func TestMatchesRule_UnknownOperator_SkipsRule(t *testing.T) {
	t.Parallel()
	rules := []dbgen.IngressRule{
		{Field: "filename", Operator: "regex", Value: ".*\\.pdf", Action: "deny"},
	}
	res := EvaluateRules(rules, ItemMeta{Filename: "report.pdf"})
	if !res.Allow {
		t.Fatal("unknown operator should be skipped (not deny)")
	}
}

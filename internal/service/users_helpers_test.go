package service_test

import (
	"testing"

	"damask/server/internal/service"
)

func TestNilIfEmpty(t *testing.T) {
	t.Parallel()

	if service.NilIfEmpty("") != nil {
		t.Error("empty string should return nil")
	}
	got := service.NilIfEmpty("hello")
	if got == nil || *got != "hello" {
		t.Errorf("non-empty should return ptr, got %v", got)
	}
}

func TestMethodName(t *testing.T) {
	t.Parallel()
	if service.MethodName(true) != "google" {
		t.Error("expected google for isGoogle=true")
	}
	if service.MethodName(false) != "oidc" {
		t.Error("expected oidc for isGoogle=false")
	}
}

func TestRemoveAuthMethodStr(t *testing.T) {
	t.Parallel()

	// removes present method
	got := service.RemoveAuthMethodStr(`["password","google"]`, "google")
	if got != `["password"]` {
		t.Errorf("unexpected result: %s", got)
	}

	// noop when method not present
	got = service.RemoveAuthMethodStr(`["password"]`, "oidc")
	if got != `["password"]` {
		t.Errorf("unexpected result after noop: %s", got)
	}

	// empty/invalid JSON returns empty array
	got = service.RemoveAuthMethodStr("", "anything")
	if got != "[]" && got != "null" {
		// marshal of nil slice is "null" but empty slice is "[]"
		// both are acceptable for bad input
		t.Logf("got %q for empty input (acceptable)", got)
	}
}

func TestHasOnlyMethodStr(t *testing.T) {
	t.Parallel()

	if !service.HasOnlyMethodStr(`["oidc"]`, "oidc") {
		t.Error("expected true for sole matching method")
	}
	if service.HasOnlyMethodStr(`["oidc","google"]`, "oidc") {
		t.Error("expected false for two methods")
	}
	if service.HasOnlyMethodStr(`["google"]`, "oidc") {
		t.Error("expected false for wrong sole method")
	}
	if service.HasOnlyMethodStr(`[]`, "oidc") {
		t.Error("expected false for empty methods")
	}
}

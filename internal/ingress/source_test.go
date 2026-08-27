package ingress

import (
	"context"
	"errors"
	"io"
	"testing"
)

// fakeSource is a minimal Source implementation for registry tests.
type fakeSource struct{ typ string }

func (f *fakeSource) Type() string                                 { return f.typ }
func (f *fakeSource) Validate(_ context.Context) error             { return nil }
func (f *fakeSource) Poll(_ context.Context) ([]IngestItem, error) { return nil, nil }
func (f *fakeSource) Fetch(_ context.Context, _ IngestItem) (io.ReadCloser, error) {
	return nil, errors.New("not implemented")
}

func TestBuild_UnknownType(t *testing.T) {
	t.Parallel()
	_, err := Build("nonexistent_type", []byte(`{}`))
	if err == nil {
		t.Fatal("expected error for unknown source type")
	}
}

func TestBuild_RegisteredType(t *testing.T) {
	t.Parallel()
	const typ = "fake_test_source"
	Register(typ, func(_ []byte) (Source, error) {
		return &fakeSource{typ: typ}, nil
	})

	src, err := Build(typ, []byte(`{}`))
	if err != nil {
		t.Fatalf("Build: %v", err)
	}
	if src.Type() != typ {
		t.Fatalf("expected type %q, got %q", typ, src.Type())
	}
}

func TestBuild_ConstructorError(t *testing.T) {
	t.Parallel()
	const typ = "bad_constructor"
	Register(typ, func(_ []byte) (Source, error) {
		return nil, errors.New("bad config")
	})

	_, err := Build(typ, []byte(`{}`))
	if err == nil {
		t.Fatal("expected error from constructor")
	}
}

func TestRegister_OverwritesExisting(t *testing.T) {
	t.Parallel()
	const typ = "overwrite_source"
	Register(typ, func(_ []byte) (Source, error) {
		return &fakeSource{typ: "original"}, nil
	})
	Register(typ, func(_ []byte) (Source, error) {
		return &fakeSource{typ: "overwritten"}, nil
	})

	src, err := Build(typ, []byte(`{}`))
	if err != nil {
		t.Fatalf("Build: %v", err)
	}
	if src.Type() != "overwritten" {
		t.Fatalf("expected overwritten constructor to win, got %q", src.Type())
	}
}

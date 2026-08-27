package service

import (
	"context"
	"errors"
	"net"
	"strings"
	"testing"

	"damask/server/internal/ai"
)

func TestClassifyModelsError_Timeout(t *testing.T) {
	got := classifyModelsError(context.DeadlineExceeded)
	if got != "timeout" {
		t.Fatalf("classifyModelsError(DeadlineExceeded) = %q, want %q", got, "timeout")
	}
}

func TestClassifyModelsError_Network(t *testing.T) {
	netErr := &net.DNSError{Err: "no such host", Name: "upstream.internal.example"}
	got := classifyModelsError(netErr)
	if got != "unreachable" {
		t.Fatalf("classifyModelsError(net error) = %q, want %q", got, "unreachable")
	}
	if strings.Contains(got, "upstream.internal.example") {
		t.Fatalf("classifyModelsError leaked internal host: %q", got)
	}
}

func TestClassifyModelsError_APIError(t *testing.T) {
	got := classifyModelsError(errors.Join(ai.ErrAPIError, errors.New("status=502 body=<html>secret trace</html>")))
	if got != modelsErrorUpstream {
		t.Fatalf("classifyModelsError(ErrAPIError) = %q, want %q", got, modelsErrorUpstream)
	}
	if strings.Contains(got, "secret trace") {
		t.Fatalf("classifyModelsError leaked response body: %q", got)
	}
}

func TestClassifyModelsError_Generic(t *testing.T) {
	got := classifyModelsError(errors.New("https://internal-proxy.corp:9443/v2/models returned garbage"))
	if got != modelsErrorUpstream {
		t.Fatalf("classifyModelsError(generic) = %q, want %q", got, modelsErrorUpstream)
	}
	if strings.Contains(got, "internal-proxy") {
		t.Fatalf("classifyModelsError leaked internal URL: %q", got)
	}
}

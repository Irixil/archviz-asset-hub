//go:build integration

package api_test

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"testing"
	"time"

	th "damask/server/internal/testhelpers"

	"github.com/gofiber/fiber/v3"
	"github.com/valyala/fasthttp"
	"github.com/valyala/fasthttp/fasthttputil"
)

// TestUploadAsset_BodyLimitExceeded verifies that uploading a file larger than
// the configured BodyLimit returns HTTP 413 with a JSON error body.
//
// app.Test() cannot be used here: Fiber's test helper surfaces
// fasthttp.ErrBodyTooLarge as a Go error and never exposes the HTTP response
// that serverErrorHandler already flushed. An in-memory TCP listener exercises
// the full connection lifecycle. A fasthttp client is used because net/http
// treats connection-close-after-response as a transport error.
func TestUploadAsset_BodyLimitExceeded(t *testing.T) {
	const smallLimit = 512 // 512 bytes — tiny limit so the test body is cheap
	env, owner := th.SetupWithOwner(t, th.WithBodyLimit(smallLimit))

	ln := fasthttputil.NewInmemoryListener()
	errCh := make(chan error, 1)
	go func() {
		errCh <- env.App.Listener(ln, fiber.ListenConfig{DisableStartupMessage: true})
	}()
	t.Cleanup(func() {
		env.App.Shutdown() //nolint:errcheck
		if err := <-errCh; err != nil && !errors.Is(err, net.ErrClosed) {
			t.Errorf("listener shutdown: %v", err)
		}
	})

	// Wait until the listener is ready.
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		c, err := ln.Dial()
		if err == nil {
			_ = c.Close()
			break
		}
		time.Sleep(5 * time.Millisecond)
	}

	body := buildMultipartBody(t, smallLimit+1) // one byte over the limit

	req := fasthttp.AcquireRequest()
	defer fasthttp.ReleaseRequest(req)
	req.SetRequestURI("http://app/api/v1/assets")
	req.Header.SetMethod(fasthttp.MethodPost)
	req.Header.SetContentType("multipart/form-data; boundary=testboundary")
	req.Header.SetCookie("auth_token", owner.Cookie.Value)
	req.SetBody(body)

	resp := fasthttp.AcquireResponse()
	defer fasthttp.ReleaseResponse(resp)

	client := &fasthttp.Client{
		Dial: func(string) (net.Conn, error) { return ln.Dial() },
	}

	if err := client.Do(req, resp); err != nil {
		t.Fatalf("do request: %v", err)
	}

	if resp.StatusCode() != fasthttp.StatusRequestEntityTooLarge {
		t.Fatalf("expected 413, got %d: %s", resp.StatusCode(), resp.Body())
	}

	var errBody map[string]string
	if err := json.Unmarshal(resp.Body(), &errBody); err != nil {
		t.Fatalf("decode response body: %v", err)
	}
	if errBody["error"] == "" {
		t.Error("expected non-empty error field in response body")
	}
}

// buildMultipartBody returns a minimal multipart body whose file payload is
// exactly size bytes. The boundary matches the Content-Type header set above.
func buildMultipartBody(t *testing.T, size int) []byte {
	t.Helper()
	const boundary = "testboundary"
	var buf bytes.Buffer
	fmt.Fprintf(&buf, "--%s\r\n", boundary)
	fmt.Fprintf(&buf, "Content-Disposition: form-data; name=\"file\"; filename=\"big.bin\"\r\n")
	fmt.Fprintf(&buf, "Content-Type: application/octet-stream\r\n\r\n")
	buf.Write(bytes.Repeat([]byte{0x00}, size))
	fmt.Fprintf(&buf, "\r\n--%s--\r\n", boundary)
	return buf.Bytes()
}

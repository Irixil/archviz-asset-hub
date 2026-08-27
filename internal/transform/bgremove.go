package transform

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"time"
)

// RemoveBgAPIURL is the Remove.bg v1 endpoint.
const RemoveBgAPIURL = "https://api.remove.bg/v1.0/removebg"

// ErrNoBgAPIKey is returned when no Remove.bg API key is configured.
var ErrNoBgAPIKey = errors.New("REMOVEBG_API_KEY is not configured")

// RemoveBackground calls the Remove.bg API to remove the background from the
// provided image bytes. Returns PNG bytes on success.
// Returns ErrNoBgAPIKey if apiKey is empty.
func (t *transformer) RemoveBackground(ctx context.Context, imageData []byte, apiKey string) ([]byte, error) {
	if apiKey == "" {
		return nil, ErrNoBgAPIKey
	}

	ctx, cancel := context.WithTimeout(ctx, 60*time.Second)
	defer cancel()

	var body bytes.Buffer
	w := multipart.NewWriter(&body)

	fw, err := w.CreateFormFile("image_file", "image.jpg")
	if err != nil {
		return nil, fmt.Errorf("create form file: %w", err)
	}
	if _, err = fw.Write(imageData); err != nil {
		return nil, fmt.Errorf("write image data: %w", err)
	}
	_ = w.WriteField("size", "auto")
	_ = w.WriteField("format", "png")
	err = w.Close()
	if err != nil {
		return nil, fmt.Errorf("close file: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, RemoveBgAPIURL, &body)
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}
	req.Header.Set("X-Api-Key", apiKey)
	req.Header.Set("Content-Type", w.FormDataContentType())

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("remove.bg request: %w", err)
	}
	defer resp.Body.Close()

	respBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("remove.bg API error %d: %s", resp.StatusCode, string(respBytes))
	}

	return respBytes, nil
}

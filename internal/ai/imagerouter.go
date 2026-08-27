package ai

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"mime/multipart"
	"net/http"
	"net/textproto"
	"strings"
	"time"

	"damask/server/internal/transform"
)

var (
	errIRNotConfigured = errors.New("imagerouter: API key not configured")
	errIRInvalidModel  = errors.New("imagerouter: model not found or not image2image")
	errIRAPI           = errors.New("imagerouter: API returned non-2xx")
)

const (
	imageRouterDefaultBaseURL = "https://api.imagerouter.io/v1"
	irClientTimeout           = 120 * time.Second
	irGenerationFailedText    = "ImageRouter generation failed"
)

type imageRouterClient struct {
	apiKey                  string
	baseURL                 string
	httpClient              *http.Client
	retryPaidOnFreeLimit429 bool
}

type imageResponseEnvelope struct {
	Created int64 `json:"created"`
	Data    []struct {
		URL           string  `json:"url"`
		B64JSON       string  `json:"b64_json"`
		RevisedPrompt *string `json:"revised_prompt"`
	} `json:"data"`
	Cost    float64 `json:"cost"`
	Latency int     `json:"latency"`
}

type errorResponseEnvelope struct {
	Status     int    `json:"status"`
	StatusText string `json:"statusText"`
	Error      struct {
		Message string `json:"message"`
		Type    string `json:"type"`
	} `json:"error"`
}

type irError struct {
	cause   error
	message string
}

func (e *irError) Error() string {
	return e.message
}

func (e *irError) Unwrap() error {
	return e.cause
}

func newImageRouterClient(apiKey, baseURL string, retryPaidOnFreeLimit429 bool) *imageRouterClient {
	return &imageRouterClient{
		apiKey:                  apiKey,
		baseURL:                 strings.TrimRight(baseURL, "/"),
		retryPaidOnFreeLimit429: retryPaidOnFreeLimit429,
		httpClient: &http.Client{
			Timeout: irClientTimeout,
		},
	}
}

func (c *imageRouterClient) bgRemove(ctx context.Context, imageData []byte, model string) ([]byte, error) {
	if strings.TrimSpace(model) == "" {
		return nil, fmt.Errorf("%w: empty model", errIRInvalidModel)
	}
	result, err := c.editImage(ctx, imageData, model, "")
	if err != nil && strings.Contains(err.Error(), "Prompt must be a string") {
		return nil, fmt.Errorf("%w: %w", ErrModelNotSupported, err)
	}
	return result, err
}

func (c *imageRouterClient) transform(ctx context.Context, imageData []byte, prompt, model string) ([]byte, error) {
	if strings.TrimSpace(model) == "" {
		return nil, fmt.Errorf("%w: empty model", errIRInvalidModel)
	}
	if strings.TrimSpace(prompt) == "" {
		return nil, errors.New("imagerouter: prompt is required")
	}
	return c.editImage(ctx, imageData, model, prompt)
}

func (c *imageRouterClient) validate(ctx context.Context) error {
	if strings.TrimSpace(c.apiKey) == "" {
		return errIRNotConfigured
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, imageRouterModelsEndpointURL(c.baseURL), nil)
	if err != nil {
		return fmt.Errorf("imagerouter: build request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+c.apiKey)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("imagerouter: request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusUnauthorized || resp.StatusCode == http.StatusForbidden {
		return ErrInvalidKey
	}
	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		payload, _ := io.ReadAll(resp.Body)
		return wrapIRError(errIRAPI, payload)
	}
	return nil
}

func (c *imageRouterClient) editImage(ctx context.Context, imageData []byte, model, prompt string) ([]byte, error) {
	if strings.TrimSpace(c.apiKey) == "" {
		return nil, errIRNotConfigured
	}

	result, statusCode, payload, err := c.performEditImage(ctx, imageData, model, prompt)
	if err == nil {
		return result, nil
	}
	if !c.shouldRetryWithoutFreeSuffix(statusCode, model, payload) {
		return nil, err
	}

	retryModel := strings.TrimSuffix(model, ":free")
	slog.InfoContext(ctx,
		"imagerouter retrying without free suffix after free-tier limit",
		"model", model,
		"retry_model", retryModel,
	)
	result, _, _, retryErr := c.performEditImage(ctx, imageData, retryModel, prompt)
	if retryErr != nil {
		return nil, retryErr
	}
	return result, nil
}

// buildEditImageRequest assembles the multipart POST request for the
// ImageRouter image-edit endpoint.
func (c *imageRouterClient) buildEditImageRequest(
	ctx context.Context,
	imageData []byte,
	model, prompt string,
) (*http.Request, error) {
	filename, contentType, err := detectIRImageUpload(imageData)
	if err != nil {
		return nil, err
	}

	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)
	if err = writer.WriteField("model", model); err != nil {
		return nil, fmt.Errorf("imagerouter: write model field: %w", err)
	}
	if err = writer.WriteField("prompt", prompt); err != nil {
		return nil, fmt.Errorf("imagerouter: write prompt field: %w", err)
	}
	header := make(textproto.MIMEHeader)
	header.Set("Content-Disposition", fmt.Sprintf(`form-data; name="image[]"; filename="%s"`, filename))
	header.Set("Content-Type", contentType)
	part, err := writer.CreatePart(header)
	if err != nil {
		return nil, fmt.Errorf("imagerouter: create image part: %w", err)
	}
	if _, err = part.Write(imageData); err != nil {
		return nil, fmt.Errorf("imagerouter: write image part: %w", err)
	}
	if err = writer.Close(); err != nil {
		return nil, fmt.Errorf("imagerouter: close multipart: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+"/openai/images/edits", body)
	if err != nil {
		return nil, fmt.Errorf("imagerouter: build request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+c.apiKey)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	return req, nil
}

func (c *imageRouterClient) performEditImage(
	ctx context.Context,
	imageData []byte,
	model, prompt string,
) ([]byte, int, string, error) {
	ctx, span := startGenAISpan(ctx, "edit-image", model, prompt)

	req, err := c.buildEditImageRequest(ctx, imageData, model, prompt)
	if err != nil {
		endGenAISpan(span, model, 0, err)
		return nil, 0, "", err
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, 0, "", fmt.Errorf("imagerouter: request failed: %w", err)
	}
	defer resp.Body.Close()

	payload, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, 0, "", fmt.Errorf("imagerouter: read response: %w", err)
	}
	payloadText := strings.TrimSpace(string(payload))

	if resp.StatusCode == http.StatusNotFound || resp.StatusCode == http.StatusUnprocessableEntity {
		apiErr := wrapIRError(errIRInvalidModel, payload)
		endGenAISpan(span, model, 0, apiErr)
		return nil, resp.StatusCode, payloadText, apiErr
	}
	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		apiErr := wrapIRError(errIRAPI, payload)
		endGenAISpan(span, model, 0, apiErr)
		return nil, resp.StatusCode, payloadText, apiErr
	}

	var envelope imageResponseEnvelope
	if err = json.Unmarshal(payload, &envelope); err != nil {
		decodeErr := parseIRError(errIRAPI, payload, fmt.Errorf("imagerouter: decode response: %w", err))
		endGenAISpan(span, model, 0, decodeErr)
		return nil, resp.StatusCode, payloadText, decodeErr
	}
	if len(envelope.Data) == 0 {
		emptyErr := parseIRError(errIRAPI, payload, &irError{
			cause:   errIRAPI,
			message: irGenerationFailedText,
		})
		endGenAISpan(span, model, 0, emptyErr)
		return nil, resp.StatusCode, payloadText, emptyErr
	}

	item := envelope.Data[0]
	if strings.TrimSpace(item.URL) != "" {
		downloaded, fetchErr := c.fetchImageURL(ctx, item.URL)
		if fetchErr != nil {
			endGenAISpan(span, model, 0, fetchErr)
			return nil, resp.StatusCode, payloadText, fetchErr
		}
		endGenAISpan(span, model, envelope.Cost, nil)
		return downloaded, resp.StatusCode, payloadText, nil
	}
	if strings.TrimSpace(item.B64JSON) == "" {
		missingErr := errors.New("imagerouter: response missing image url")
		endGenAISpan(span, model, 0, missingErr)
		return nil, resp.StatusCode, payloadText, missingErr
	}
	decoded, err := base64.StdEncoding.DecodeString(item.B64JSON)
	if err != nil {
		decodeErr := fmt.Errorf("imagerouter: decode image: %w", err)
		endGenAISpan(span, model, 0, decodeErr)
		return nil, resp.StatusCode, payloadText, decodeErr
	}
	endGenAISpan(span, model, envelope.Cost, nil)
	return decoded, resp.StatusCode, payloadText, nil
}

func wrapIRError(base error, payload []byte) error {
	return parseIRError(base, payload, nil)
}

func parseIRError(base error, payload []byte, fallback error) error {
	var envelope errorResponseEnvelope
	if err := json.Unmarshal(payload, &envelope); err != nil {
		if fallback != nil {
			return fallback
		}
		return &irError{cause: base, message: irGenerationFailedText}
	}
	if msg := strings.TrimSpace(envelope.Error.Message); msg != "" {
		return &irError{cause: base, message: msg}
	}
	if fallback != nil {
		return fallback
	}
	return &irError{cause: base, message: irGenerationFailedText}
}

func (c *imageRouterClient) fetchImageURL(ctx context.Context, rawURL string) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, rawURL, nil)
	if err != nil {
		return nil, fmt.Errorf("imagerouter: build image download request: %w", err)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("imagerouter: download image: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		payload, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf(
			"imagerouter: image download failed: status=%d body=%s",
			resp.StatusCode,
			strings.TrimSpace(string(payload)),
		)
	}

	downloaded, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("imagerouter: read downloaded image: %w", err)
	}
	if len(downloaded) == 0 {
		return nil, errors.New("imagerouter: downloaded image is empty")
	}
	return downloaded, nil
}

func detectIRImageUpload(imageData []byte) (filename string, contentType string, err error) {
	if len(imageData) == 0 {
		return "", "", errors.New("imagerouter: source image is empty")
	}

	switch http.DetectContentType(imageData) {
	case transform.MimeImagePNG:
		return "source.png", transform.MimeImagePNG, nil
	case transform.MimeImageJPEG:
		return "source.jpg", transform.MimeImageJPEG, nil
	case transform.MimeImageWebP:
		return "source.webp", transform.MimeImageWebP, nil
	case transform.MimeImageGIF:
		return "source.gif", transform.MimeImageGIF, nil
	default:
		return "", "", fmt.Errorf(
			"imagerouter: unsupported source image format %q; expected PNG, JPEG, or WEBP",
			http.DetectContentType(imageData),
		)
	}
}

func (c *imageRouterClient) shouldRetryWithoutFreeSuffix(statusCode int, model, payload string) bool {
	if !c.retryPaidOnFreeLimit429 {
		return false
	}
	if statusCode != http.StatusTooManyRequests {
		return false
	}
	if !strings.HasSuffix(model, ":free") {
		return false
	}

	lowerPayload := strings.ToLower(payload)
	return strings.Contains(lowerPayload, "remove \":free\"") ||
		(strings.Contains(lowerPayload, "free requests reached") && strings.Contains(lowerPayload, "daily limit"))
}

// --- Provider implementation ---

type imageRouterProvider struct {
	client     *imageRouterClient
	apiKey     string
	keySource  KeySource
	configured bool
}

// NewImageRouterProvider returns a Provider backed by imagerouter.io.
// Pass apiKey="" and keySource="none" when not configured;
// the provider will be registered but skipped by the resolver.
func NewImageRouterProvider(apiKey string, keySource KeySource, retryPaidOnFreeLimit bool) Provider {
	return newImageRouterProviderWithBaseURL(apiKey, keySource, retryPaidOnFreeLimit, imageRouterDefaultBaseURL)
}

// NewImageRouterProviderForTest constructs an imageRouterProvider with a custom base URL,
// allowing tests to point at a local [httptest.Server] without a package-level mutable var.
func NewImageRouterProviderForTest(
	apiKey string,
	keySource KeySource,
	retryPaidOnFreeLimit bool,
	baseURL string,
) Provider {
	return newImageRouterProviderWithBaseURL(apiKey, keySource, retryPaidOnFreeLimit, baseURL)
}

func newImageRouterProviderWithBaseURL(
	apiKey string,
	keySource KeySource,
	retryPaidOnFreeLimit bool,
	baseURL string,
) Provider {
	configured := apiKey != ""
	return &imageRouterProvider{
		client:     newImageRouterClient(apiKey, baseURL, retryPaidOnFreeLimit),
		apiKey:     apiKey,
		keySource:  keySource,
		configured: configured,
	}
}

func (p *imageRouterProvider) ID() ProviderID { return ProviderImageRouter }

func (p *imageRouterProvider) Capabilities() Capability {
	return CapBgRemove | CapImageToImage
}

func (p *imageRouterProvider) IsConfigured() bool   { return p.configured }
func (p *imageRouterProvider) KeySource() KeySource { return p.keySource }

func (p *imageRouterProvider) BgRemove(ctx context.Context, imageData []byte, model string) ([]byte, error) {
	return p.client.bgRemove(ctx, imageData, model)
}

func (p *imageRouterProvider) Transform(ctx context.Context, imageData []byte, prompt, model string) ([]byte, error) {
	return p.client.transform(ctx, imageData, prompt, model)
}

func (p *imageRouterProvider) ValidateKey(ctx context.Context) error {
	return p.client.validate(ctx)
}

// DescribeImage is not supported by ImageRouter. It declares neither
// CapVisionTag nor CapImageDescription, so this is never reached through
// the resolver — it exists only to satisfy the Provider interface.
func (p *imageRouterProvider) DescribeImage(
	_ context.Context,
	_, _ string,
	_ []byte,
	_ string,
) (string, error) {
	return "", errors.New("imagerouter: DescribeImage not supported")
}

// TranscribeAudio is not supported by ImageRouter. It does not declare
// CapAudioTranscription, so this is never reached through the resolver —
// it exists only to satisfy the Provider interface.
func (p *imageRouterProvider) TranscribeAudio(_ context.Context, _ string, _ []byte, _ string) (string, error) {
	return "", errors.New("imagerouter: TranscribeAudio not supported")
}

// TagText is not supported by ImageRouter. It does not declare CapTextTag,
// so this is never reached through the resolver — it exists only to satisfy
// the Provider interface.
func (p *imageRouterProvider) TagText(_ context.Context, _, _ string) (string, error) {
	return "", errors.New("imagerouter: TagText not supported")
}

func (p *imageRouterProvider) ListModels(ctx context.Context) ([]Model, error) {
	const cacheKey = string(ProviderImageRouter)
	if cached, ok := modelCache.Get(cacheKey); ok {
		return cached, nil
	}

	irModels, err := fetchImageRouterModels(ctx, p.apiKey, p.client.baseURL)
	if err != nil {
		return nil, err
	}
	out := make([]Model, len(irModels))
	for i, m := range irModels {
		out[i] = Model{
			ID:            m.ID,
			Name:          m.Name,
			ProviderID:    ProviderImageRouter,
			PricePerImage: m.PricePerImage,
			Capabilities:  m.Capabilities,
		}
	}
	modelCache.Set(cacheKey, out, 0)
	return out, nil
}

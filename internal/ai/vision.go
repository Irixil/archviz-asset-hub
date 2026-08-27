package ai

// DefaultVisionModel is the OpenRouter model used for AI image description
// when the user does not specify one.
const DefaultVisionModel = "google/gemini-2.5-flash"

// DefaultImageDescriptionPrompt is the prompt used when the user does not
// provide a custom one.
const DefaultImageDescriptionPrompt = "Describe this image in detail, including its subject, composition, colors, and any visible text. Be thorough."

// MaxImageDescriptionPromptLen caps user-supplied prompt length to bound
// cost and avoid blowing past the model's context window.
const MaxImageDescriptionPromptLen = 2000

// MaxDescribeImageBytes caps the source image size read into memory and
// base64-encoded for the vision API request.
const MaxDescribeImageBytes = 50 * 1024 * 1024

// VisionModel is a curated OpenRouter vision model for display in the UI.
type VisionModel struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

// CuratedVisionModels is the model list shown in the AI description form.
// Ordered by quality/cost tradeoff.
//
//nolint:gochecknoglobals // intentional package-level list; read-only after init
var CuratedVisionModels = []VisionModel{
	{ID: "google/gemini-2.5-flash", Name: "Gemini 2.5 Flash (Google)"},
	{ID: "google/gemini-2.5-pro", Name: "Gemini 2.5 Pro (Google)"},
	{ID: "openai/gpt-4o-mini", Name: "GPT-4o Mini (OpenAI)"},
	{ID: "openai/gpt-4o", Name: "GPT-4o (OpenAI)"},
	{ID: "anthropic/claude-haiku-4.5", Name: "Claude Haiku 4.5 (Anthropic)"},
	{ID: "anthropic/claude-sonnet-4.6", Name: "Claude Sonnet 4.6 (Anthropic)"},
	{ID: "meta-llama/llama-3.2-11b-vision-instruct", Name: "Llama 3.2 Vision 11B (Meta)"},
}

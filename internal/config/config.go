// Package config loads and exposes application configuration.
package config

import (
	"errors"
	"fmt"
	"log/slog"
	"net/url"
	"os"
	"strconv"
	"strings"

	"damask/server/internal/mail"
	"damask/server/internal/storage"

	"github.com/joho/godotenv"
)

const defaultSMTPPort = 25
const defaultPurgeHour = 3
const defaultQueueWorkers = 4
const defaultSFTPPort = 22
const defaultDemoResetHours = 6

// OIDCConfig holds config for a generic OIDC provider (selfhosted IDP).
// Provider and Verifier are populated at startup via OIDC discovery.
type OIDCConfig struct {
	IssuerURL    string
	ClientID     string
	ClientSecret string
	Label        string // display label on the login page
}

// GoogleOIDCConfig holds config for Google OIDC login.
type GoogleOIDCConfig struct {
	Auth         bool
	ClientID     string
	ClientSecret string
}

// CanvaConfig holds config for Canva OAuth login and import.
type CanvaConfig struct {
	Auth         bool
	ClientID     string
	ClientSecret string
}

// ImageRouterConfig holds credentials and default model selection for the
// imagerouter.io API.
type ImageRouterConfig struct {
	APIKey               string
	DefaultModel         string
	DefaultBgRemoveModel string
	RetryPaidOnFreeLimit bool
}

// OpenRouterConfig holds credentials and default model selection for the
// openrouter.ai API.
type OpenRouterConfig struct {
	APIKey         string
	DefaultModel   string
	DefaultBgModel string
}

// IsConfigured reports whether an OpenRouter API key is set.
func (c OpenRouterConfig) IsConfigured() bool { return c.APIKey != "" }

func loadOpenRouterConfig() OpenRouterConfig {
	return OpenRouterConfig{
		APIKey:         os.Getenv("OPENROUTER_API_KEY"),
		DefaultModel:   getEnv("OPENROUTER_DEFAULT_MODEL", "openai/dall-e-2"),
		DefaultBgModel: getEnv("OPENROUTER_DEFAULT_BG_REMOVE_MODEL", "stability-ai/stable-diffusion-xl-refiner"),
	}
}

func loadDemoConfig(demoMode bool) DemoConfig {
	return DemoConfig{
		DemoMode:           demoMode,
		ResetIntervalHours: getEnvPositiveInt("DEMO_RESET_INTERVAL_HOURS", defaultDemoResetHours),
		UserEmail:          getEnv("DEMO_USER_EMAIL", "demo@damask.studio"),
		WorkspaceName:      getEnv("DEMO_WORKSPACE_NAME", "Demo Agency"),
		ShowBanner: demoMode &&
			getEnv("DEMO_BANNER", strconv.FormatBool(true)) != strconv.FormatBool(false),
		SignupURL: getEnv("DEMO_SIGNUP_URL", "/signup"),
	}
}

func loadOIDCConfig() OIDCConfig {
	return OIDCConfig{
		IssuerURL:    os.Getenv("OIDC_ISSUER_URL"),
		ClientID:     os.Getenv("OIDC_CLIENT_ID"),
		ClientSecret: os.Getenv("OIDC_CLIENT_SECRET"),
		Label:        getEnv("OIDC_LABEL", "Sign in with SSO"),
	}
}

func loadGoogleOIDCConfig() GoogleOIDCConfig {
	clientID := os.Getenv("GOOGLE_CLIENT_ID")
	return GoogleOIDCConfig{
		Auth: clientID != "" &&
			getEnv("GOOGLE_SIGNIN", strconv.FormatBool(false)) == strconv.FormatBool(true),
		ClientID:     clientID,
		ClientSecret: os.Getenv("GOOGLE_CLIENT_SECRET"),
	}
}

func loadCanvaConfig() CanvaConfig {
	clientID := os.Getenv("CANVA_CLIENT_ID")
	return CanvaConfig{
		Auth: clientID != "" &&
			getEnv("CANVA_SIGNIN", strconv.FormatBool(false)) == strconv.FormatBool(true),
		ClientID:     clientID,
		ClientSecret: os.Getenv("CANVA_CLIENT_SECRET"),
	}
}

// getEnvPositiveInt reads an env var as an int, falling back to defaultVal
// if unset, unparsable, or not strictly positive.
func getEnvPositiveInt(key string, defaultVal int) int {
	if v := os.Getenv(key); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			return n
		}
	}
	return defaultVal
}

type FFmpegConfig struct {
	Path    string
	HWAccel string
}

// ScratchConfig holds settings for the variant draft scratch storage.
type ScratchConfig struct {
	// PurgeTime is the wall-clock time (HH:MM, 24h, UTC) at which the daily
	// scratch purge job runs. Defaults to "03:00".
	PurgeTime string
}

type Config struct {
	MailServerPort   string
	MailServerHost   string
	MailSenderConfig mail.Config
	Port             string
	DBPath           string
	StoragePath      string
	StorageType      string // "local" | "sftp"
	StorageSFTP      storage.SFTPConfig
	StorageS3        storage.AferoS3Config
	JWTSecret        string
	AppSecret        string
	AppEnv           string
	BaseURL          *url.URL
	QueueWorkers     int
	FrontendPath     string
	EnableScheduler  bool
	EnableSignup     bool
	Demo             DemoConfig
	// BodyLimit overrides the default 100 MB upload limit. Zero means use the default.
	BodyLimit int

	OIDC   OIDCConfig
	Google GoogleOIDCConfig
	Canva  CanvaConfig

	ImageRouter ImageRouterConfig
	OpenRouter  OpenRouterConfig
	FFmpeg      FFmpegConfig
	Scratch     ScratchConfig

	Telemetry TelemetryConfig
}

type TelemetryConfig struct {
	Enabled     bool
	Endpoint    string
	Token       string
	ServiceName string
	Env         string
}

// DemoConfig holds all demo-mode settings. All fields are zero-valued when
// DemoMode is false.
type DemoConfig struct {
	DemoMode           bool
	ResetIntervalHours int
	UserEmail          string
	WorkspaceName      string
	ShowBanner         bool
	SignupURL          string
}

func Load() (*Config, error) {
	// Load .env file in development (ignore error if file doesn't exist)
	_ = godotenv.Load(".env")

	workers := getEnvPositiveInt("QUEUE_WORKERS", defaultQueueWorkers)
	sftpPort := getEnvPositiveInt("STORAGE_SFTP_PORT", defaultSFTPPort)
	demoMode := getEnv("DEMO_MODE", strconv.FormatBool(false)) == strconv.FormatBool(true)

	cfg := &Config{
		Port:           getEnv("PORT", "8080"),
		MailServerPort: getEnv("MAIL_PORT", "2525"),
		DBPath:         getEnv("DB_PATH", "./damask.db"),
		StoragePath:    getEnv("STORAGE_LOCAL_PATH", "./storage"),
		StorageType:    getEnv("STORAGE", "local"),
		MailSenderConfig: mail.Config{
			Host:     os.Getenv("SMTP_HOST"),
			Port:     getEnvInt("SMTP_PORT", defaultSMTPPort),
			Sender:   os.Getenv("SMTP_SENDER"),
			User:     os.Getenv("SMTP_USER"),
			Password: os.Getenv("SMTP_PASS"),
		},
		StorageSFTP: storage.SFTPConfig{
			Host:            os.Getenv("STORAGE_SFTP_HOST"),
			Port:            sftpPort,
			User:            os.Getenv("STORAGE_SFTP_USER"),
			AuthMethod:      getEnv("STORAGE_SFTP_AUTH_METHOD", "password"),
			Password:        os.Getenv("STORAGE_SFTP_PASSWORD"),
			PrivateKey:      os.Getenv("STORAGE_SFTP_PRIVATE_KEY"),
			BasePath:        getEnv("STORAGE_SFTP_BASE_PATH", "/"),
			InsecureHostKey: os.Getenv("STORAGE_SFTP_INSECURE_HOST_KEY") == strconv.FormatBool(true),
		},
		StorageS3: storage.AferoS3Config{
			Base:      getEnv("STORAGE_S3_BASE_PATH", "/"),
			Bucket:    os.Getenv("STORAGE_S3_BUCKET"),
			Region:    os.Getenv("STORAGE_S3_REGION"),
			AccessKey: os.Getenv("STORAGE_S3_ACCESSKEY"),
			SecretKey: os.Getenv("STORAGE_S3_SECRETKEY"),
		},
		JWTSecret:       os.Getenv("JWT_SECRET"),
		AppSecret:       os.Getenv("APP_SECRET"),
		AppEnv:          getEnv("APP_ENV", "development"),
		QueueWorkers:    workers,
		FrontendPath:    os.Getenv("FRONTEND_PATH"),
		EnableScheduler: getEnv("ENABLE_SCHEDULER", strconv.FormatBool(true)) != strconv.FormatBool(false),
		EnableSignup:    getEnv("ENABLE_SIGNUP", strconv.FormatBool(true)) != strconv.FormatBool(false),
		Demo:            loadDemoConfig(demoMode),
		ImageRouter: ImageRouterConfig{
			APIKey:               os.Getenv("IMAGEROUTER_API_KEY"),
			DefaultModel:         getEnv("IMAGEROUTER_DEFAULT_MODEL", "black-forest-labs/FLUX-2-klein-4b:free"),
			DefaultBgRemoveModel: getEnv("IMAGEROUTER_DEFAULT_BG_REMOVE_MODEL", "bria/remove-background:free"),
			RetryPaidOnFreeLimit: getEnv(
				"IMAGEROUTER_RETRY_PAID_ON_FREE_LIMIT",
				strconv.FormatBool(false),
			) == strconv.FormatBool(
				true,
			),
		},
		OpenRouter: loadOpenRouterConfig(),
		FFmpeg: FFmpegConfig{
			Path:    strings.TrimSpace(os.Getenv("FFMPEG_PATH")),
			HWAccel: strings.ToLower(strings.TrimSpace(os.Getenv("FFMPEG_HW_ACCEL"))),
		},
	}
	scratchPurgeTime := getEnv("SCRATCH_PURGE_TIME", "03:00")
	cfg.Scratch = ScratchConfig{PurgeTime: scratchPurgeTime}
	if _, _, ok := parsePurgeTime(scratchPurgeTime); !ok {
		return nil, fmt.Errorf("SCRATCH_PURGE_TIME %q is invalid; expected HH:MM (24h UTC)", scratchPurgeTime)
	}
	cfg.Telemetry = TelemetryConfig{
		Enabled:     getEnv("OTEL_ENABLED", strconv.FormatBool(false)) == strconv.FormatBool(true),
		Endpoint:    getEnv("OTEL_ENDPOINT", "http://localhost:8082/api/otel/v1"),
		Token:       getEnv("OTEL_TOKEN", "dev-token"),
		ServiceName: "damask",
		Env:         cfg.AppEnv,
	}
	cfg.OIDC = loadOIDCConfig()
	cfg.Google = loadGoogleOIDCConfig()
	cfg.Canva = loadCanvaConfig()

	if cfg.JWTSecret == "" {
		return nil, errors.New("JWT_SECRET env var is required")
	}

	if cfg.AppSecret == "" {
		return nil, errors.New("APP_SECRET env var is required")
	}

	switch cfg.FFmpeg.HWAccel {
	case "", "videotoolbox", "vaapi", "qsv", "cuda":
	default:
		return nil, errors.New("FFMPEG_HW_ACCEL must be one of: videotoolbox, vaapi, qsv, cuda")
	}

	baseURL, err := url.Parse(getEnv("BASE_URL", "http://localhost:5173"))
	if err != nil {
		return nil, errors.New("BASE_URL env var is required")
	}
	cfg.BaseURL = baseURL
	cfg.MailSenderConfig.BaseURL = baseURL.String()

	mailHost := strings.TrimSpace(os.Getenv("MAIL_HOST"))
	if len(mailHost) == 0 {
		mailHost = "ingress." + baseURL.Host
	}
	cfg.MailServerHost = mailHost

	return cfg, nil
}

func getEnv(key, defaultVal string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return defaultVal
}

// parsePurgeTime parses "HH:MM" into (hour, minute, ok).
func parsePurgeTime(s string) (int, int, bool) {
	parts := strings.SplitN(s, ":", 2)
	if len(parts) != 2 { //nolint:mnd // expect a format like "03:00".
		return 0, 0, false
	}
	h, errH := strconv.Atoi(parts[0])
	m, errM := strconv.Atoi(parts[1])
	if errH != nil || errM != nil || h < 0 || h > 23 || m < 0 || m > 59 {
		return 0, 0, false
	}
	return h, m, true
}

// PurgeHourMinute parses PurgeTime ("HH:MM") into (hour, minute).
// Panics if the value is invalid — Load() validates it at startup so this is safe.
func (c ScratchConfig) PurgeHourMinute() (int, int) {
	h, m, ok := parsePurgeTime(c.PurgeTime)
	if !ok {
		slog.Warn("SCRATCH_PURGE_TIME: invalid, using 03:00", "value", c.PurgeTime)
		return defaultPurgeHour, 0
	}
	return h, m
}

func getEnvInt(key string, defaultVal int) int {
	var err error
	i := defaultVal
	if v := os.Getenv(key); v != "" {
		if i, err = strconv.Atoi(v); err != nil {
			slog.Error("failed to parse SMTP port")
		}
	}
	return i
}

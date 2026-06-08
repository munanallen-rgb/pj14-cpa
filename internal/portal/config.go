package portal

import (
	"fmt"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"
)

const (
	defaultPortalBindAddress = "0.0.0.0:18100"
	defaultPortalDBHost      = "sub2api-postgres"
	defaultPortalDBPort      = 5432
	defaultPortalDBUser      = "sub2api"
	defaultPortalDBName      = "sub2api"
	defaultPortalDBSSLMode   = "disable"

	defaultSub2APIBaseURL      = "http://sub2api:8080"
	defaultSub2APIGroupName    = "cpa-openai"
	defaultSub2APIKeyQuota     = 0
	defaultSessionTTLHours     = 24
	defaultUsageRecordLimit    = 100
	defaultMaxUsageRecordLimit = 500
)

// ShutdownTimeout bounds graceful HTTP server shutdown.
const ShutdownTimeout = 10 * time.Second

// Config holds runtime settings for the user-facing Portal API.
type Config struct {
	BindAddress          string
	PublicSub2APIBaseURL string
	SessionSecret        string
	SessionTTL           time.Duration
	CookieSecure         bool
	AllowedOrigins       []string
	BootstrapAdmin       BootstrapAdminConfig
	Database             DatabaseConfig
	Sub2API              Sub2APIConfig
}

// BootstrapAdminConfig describes the optional first admin account.
type BootstrapAdminConfig struct {
	Email    string
	Password string
}

// DatabaseConfig describes the Postgres target used by Portal.
type DatabaseConfig struct {
	Host     string
	Port     int
	User     string
	Password string
	Name     string
	SSLMode  string
}

// Sub2APIConfig describes the Sub2API admin and runtime integration.
type Sub2APIConfig struct {
	BaseURL         string
	AdminEmail      string
	AdminPassword   string
	DefaultGroup    string
	DefaultKeyQuota float64
}

// LoadConfigFromEnv reads Portal API configuration from environment variables.
func LoadConfigFromEnv() (Config, error) {
	dbPort, errDBPort := strconv.Atoi(envOrDefault("PORTAL_DATABASE_PORT", strconv.Itoa(defaultPortalDBPort)))
	if errDBPort != nil || dbPort <= 0 {
		return Config{}, fmt.Errorf("invalid PORTAL_DATABASE_PORT")
	}

	sessionTTLHours, errTTL := strconv.Atoi(envOrDefault("PORTAL_SESSION_TTL_HOURS", strconv.Itoa(defaultSessionTTLHours)))
	if errTTL != nil || sessionTTLHours <= 0 {
		return Config{}, fmt.Errorf("invalid PORTAL_SESSION_TTL_HOURS")
	}

	keyQuota, errQuota := strconv.ParseFloat(envOrDefault("SUB2API_DEFAULT_KEY_QUOTA", strconv.Itoa(defaultSub2APIKeyQuota)), 64)
	if errQuota != nil || keyQuota < 0 {
		return Config{}, fmt.Errorf("invalid SUB2API_DEFAULT_KEY_QUOTA")
	}

	cfg := Config{
		BindAddress:          envOrDefault("PORTAL_BIND", defaultPortalBindAddress),
		PublicSub2APIBaseURL: strings.TrimRight(envOrDefault("PORTAL_PUBLIC_SUB2API_BASE_URL", ""), "/"),
		SessionSecret:        strings.TrimSpace(os.Getenv("PORTAL_SESSION_SECRET")),
		SessionTTL:           time.Duration(sessionTTLHours) * time.Hour,
		CookieSecure:         envBool("PORTAL_COOKIE_SECURE", false),
		AllowedOrigins:       splitCSV(os.Getenv("PORTAL_ALLOWED_ORIGINS")),
		BootstrapAdmin: BootstrapAdminConfig{
			Email:    normalizeEmail(os.Getenv("PORTAL_BOOTSTRAP_ADMIN_EMAIL")),
			Password: strings.TrimSpace(os.Getenv("PORTAL_BOOTSTRAP_ADMIN_PASSWORD")),
		},
		Database: DatabaseConfig{
			Host:     envOrDefault("PORTAL_DATABASE_HOST", defaultPortalDBHost),
			Port:     dbPort,
			User:     envOrDefault("PORTAL_DATABASE_USER", defaultPortalDBUser),
			Password: strings.TrimSpace(os.Getenv("PORTAL_DATABASE_PASSWORD")),
			Name:     envOrDefault("PORTAL_DATABASE_NAME", defaultPortalDBName),
			SSLMode:  envOrDefault("PORTAL_DATABASE_SSLMODE", defaultPortalDBSSLMode),
		},
		Sub2API: Sub2APIConfig{
			BaseURL:         strings.TrimRight(envOrDefault("SUB2API_BASE_URL", defaultSub2APIBaseURL), "/"),
			AdminEmail:      normalizeEmail(os.Getenv("SUB2API_ADMIN_EMAIL")),
			AdminPassword:   strings.TrimSpace(os.Getenv("SUB2API_ADMIN_PASSWORD")),
			DefaultGroup:    strings.TrimSpace(envOrDefault("SUB2API_DEFAULT_GROUP_NAME", defaultSub2APIGroupName)),
			DefaultKeyQuota: keyQuota,
		},
	}
	if cfg.PublicSub2APIBaseURL == "" {
		cfg.PublicSub2APIBaseURL = cfg.Sub2API.BaseURL
	}
	if cfg.SessionSecret == "" {
		return Config{}, fmt.Errorf("PORTAL_SESSION_SECRET is required")
	}
	if cfg.Database.Password == "" {
		return Config{}, fmt.Errorf("PORTAL_DATABASE_PASSWORD is required")
	}
	if cfg.Sub2API.AdminEmail == "" {
		return Config{}, fmt.Errorf("SUB2API_ADMIN_EMAIL is required")
	}
	if cfg.Sub2API.AdminPassword == "" {
		return Config{}, fmt.Errorf("SUB2API_ADMIN_PASSWORD is required")
	}
	if cfg.Sub2API.DefaultGroup == "" {
		return Config{}, fmt.Errorf("SUB2API_DEFAULT_GROUP_NAME is required")
	}
	if (cfg.BootstrapAdmin.Email == "") != (cfg.BootstrapAdmin.Password == "") {
		return Config{}, fmt.Errorf("PORTAL_BOOTSTRAP_ADMIN_EMAIL and PORTAL_BOOTSTRAP_ADMIN_PASSWORD must be set together")
	}
	if cfg.BootstrapAdmin.Password != "" && len(cfg.BootstrapAdmin.Password) < minPasswordLength {
		return Config{}, fmt.Errorf("PORTAL_BOOTSTRAP_ADMIN_PASSWORD must be at least %d characters", minPasswordLength)
	}
	return cfg, nil
}

// DSN returns a pgx-compatible Postgres connection string.
func (c DatabaseConfig) DSN() string {
	q := url.Values{}
	q.Set("sslmode", c.SSLMode)
	u := url.URL{
		Scheme:   "postgres",
		User:     url.UserPassword(c.User, c.Password),
		Host:     fmt.Sprintf("%s:%d", c.Host, c.Port),
		Path:     "/" + c.Name,
		RawQuery: q.Encode(),
	}
	return u.String()
}

func envOrDefault(name string, fallback string) string {
	if value := strings.TrimSpace(os.Getenv(name)); value != "" {
		return value
	}
	return fallback
}

func envBool(name string, fallback bool) bool {
	value := strings.TrimSpace(strings.ToLower(os.Getenv(name)))
	if value == "" {
		return fallback
	}
	return value == "1" || value == "true" || value == "yes" || value == "on"
}

func splitCSV(value string) []string {
	parts := strings.Split(value, ",")
	out := make([]string, 0, len(parts))
	for _, part := range parts {
		trimmed := strings.TrimSpace(part)
		if trimmed != "" {
			out = append(out, trimmed)
		}
	}
	return out
}

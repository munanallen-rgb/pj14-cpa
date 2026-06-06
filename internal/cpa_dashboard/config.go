package cpadashboard

import (
	"fmt"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"
)

const (
	defaultBindAddress = "0.0.0.0:18090"
	defaultDBHost      = "sub2api-postgres"
	defaultDBPort      = 5432
	defaultDBUser      = "sub2api_dashboard"
	defaultDBName      = "sub2api"
	defaultDBSSLMode   = "disable"
)

// ShutdownTimeout bounds graceful HTTP server shutdown.
const ShutdownTimeout = 10 * time.Second

// Config holds runtime settings for the CPA dashboard.
type Config struct {
	BindAddress   string
	LoginPassword string
	Database      DatabaseConfig
}

// DatabaseConfig describes the read-only Postgres target used by the dashboard.
type DatabaseConfig struct {
	Host     string
	Port     int
	User     string
	Password string
	Name     string
	SSLMode  string
}

// LoadConfigFromEnv reads dashboard configuration from environment variables.
func LoadConfigFromEnv() (Config, error) {
	port, errPort := strconv.Atoi(envOrDefault("CPA_DASHBOARD_DATABASE_PORT", strconv.Itoa(defaultDBPort)))
	if errPort != nil || port <= 0 {
		return Config{}, fmt.Errorf("invalid CPA_DASHBOARD_DATABASE_PORT")
	}

	cfg := Config{
		BindAddress:   envOrDefault("CPA_DASHBOARD_BIND", defaultBindAddress),
		LoginPassword: strings.TrimSpace(os.Getenv("CPA_DASHBOARD_LOGIN_PASSWORD")),
		Database: DatabaseConfig{
			Host:     envOrDefault("CPA_DASHBOARD_DATABASE_HOST", defaultDBHost),
			Port:     port,
			User:     envOrDefault("CPA_DASHBOARD_DATABASE_USER", defaultDBUser),
			Password: strings.TrimSpace(os.Getenv("CPA_DASHBOARD_DATABASE_PASSWORD")),
			Name:     envOrDefault("CPA_DASHBOARD_DATABASE_NAME", defaultDBName),
			SSLMode:  envOrDefault("CPA_DASHBOARD_DATABASE_SSLMODE", defaultDBSSLMode),
		},
	}
	if cfg.LoginPassword == "" {
		return Config{}, fmt.Errorf("CPA_DASHBOARD_LOGIN_PASSWORD is required")
	}
	if cfg.Database.Password == "" {
		return Config{}, fmt.Errorf("CPA_DASHBOARD_DATABASE_PASSWORD is required")
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

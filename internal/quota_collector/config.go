package quotacollector

import (
	"fmt"
	"net/url"
	"os"
	"strconv"
	"strings"
)

const (
	defaultInstances = "cpa1=http://cpa1:8317,cpa2=http://cpa2:8317,cpa3=http://cpa3:8317"
	defaultDBHost    = "sub2api-postgres"
	defaultDBPort    = 5432
	defaultDBUser    = "sub2api"
	defaultDBName    = "sub2api"
	defaultDBSSLMode = "disable"
)

// Config holds runtime settings for the quota collector command.
type Config struct {
	Instances []InstanceConfig
	Database  DatabaseConfig
}

// InstanceConfig describes one CPA management API target.
type InstanceConfig struct {
	Name          string
	BaseURL       string
	ManagementKey string
}

// DatabaseConfig describes the Postgres target used for snapshots.
type DatabaseConfig struct {
	Host     string
	Port     int
	User     string
	Password string
	Name     string
	SSLMode  string
}

// LoadConfigFromEnv reads collector configuration from environment variables.
func LoadConfigFromEnv() (Config, error) {
	defaultKey := strings.TrimSpace(os.Getenv("CPA_QUOTA_COLLECTOR_MANAGEMENT_KEY"))
	instances, errInstances := parseInstances(envOrDefault("CPA_QUOTA_COLLECTOR_INSTANCES", defaultInstances), defaultKey)
	if errInstances != nil {
		return Config{}, errInstances
	}
	port, errPort := strconv.Atoi(envOrDefault("CPA_QUOTA_DATABASE_PORT", strconv.Itoa(defaultDBPort)))
	if errPort != nil || port <= 0 {
		return Config{}, fmt.Errorf("invalid CPA_QUOTA_DATABASE_PORT")
	}
	cfg := Config{
		Instances: instances,
		Database: DatabaseConfig{
			Host:     envOrDefault("CPA_QUOTA_DATABASE_HOST", defaultDBHost),
			Port:     port,
			User:     envOrDefault("CPA_QUOTA_DATABASE_USER", defaultDBUser),
			Password: strings.TrimSpace(os.Getenv("CPA_QUOTA_DATABASE_PASSWORD")),
			Name:     envOrDefault("CPA_QUOTA_DATABASE_NAME", defaultDBName),
			SSLMode:  envOrDefault("CPA_QUOTA_DATABASE_SSLMODE", defaultDBSSLMode),
		},
	}
	if cfg.Database.Password == "" {
		return Config{}, fmt.Errorf("CPA_QUOTA_DATABASE_PASSWORD is required")
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

func parseInstances(raw string, defaultKey string) ([]InstanceConfig, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil, fmt.Errorf("CPA_QUOTA_COLLECTOR_INSTANCES is required")
	}
	parts := strings.Split(raw, ",")
	out := make([]InstanceConfig, 0, len(parts))
	seen := make(map[string]struct{}, len(parts))
	for _, part := range parts {
		name, baseURL, ok := strings.Cut(strings.TrimSpace(part), "=")
		name = strings.TrimSpace(name)
		baseURL = strings.TrimRight(strings.TrimSpace(baseURL), "/")
		if !ok || name == "" || baseURL == "" {
			return nil, fmt.Errorf("invalid CPA_QUOTA_COLLECTOR_INSTANCES item %q", part)
		}
		if _, errParse := url.ParseRequestURI(baseURL); errParse != nil {
			return nil, fmt.Errorf("invalid base URL for %s: %w", name, errParse)
		}
		if _, dup := seen[name]; dup {
			return nil, fmt.Errorf("duplicate CPA instance name %s", name)
		}
		seen[name] = struct{}{}
		key := instanceManagementKey(name, defaultKey)
		if key == "" {
			return nil, fmt.Errorf("management key is required for %s", name)
		}
		out = append(out, InstanceConfig{Name: name, BaseURL: baseURL, ManagementKey: key})
	}
	return out, nil
}

func instanceManagementKey(name string, defaultKey string) string {
	envName := "CPA_QUOTA_COLLECTOR_MANAGEMENT_KEY_" + strings.ToUpper(strings.ReplaceAll(name, "-", "_"))
	if key := strings.TrimSpace(os.Getenv(envName)); key != "" {
		return key
	}
	return defaultKey
}

func envOrDefault(name string, fallback string) string {
	if value := strings.TrimSpace(os.Getenv(name)); value != "" {
		return value
	}
	return fallback
}

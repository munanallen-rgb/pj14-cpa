package portal

import "testing"

func TestLoadConfigFromEnv(t *testing.T) {
	t.Setenv("PORTAL_SESSION_SECRET", "session-secret")
	t.Setenv("PORTAL_DATABASE_PASSWORD", "db-password")
	t.Setenv("SUB2API_ADMIN_EMAIL", "Admin@Example.com")
	t.Setenv("SUB2API_ADMIN_PASSWORD", "admin-password")
	t.Setenv("PORTAL_BOOTSTRAP_ADMIN_EMAIL", "Owner@Example.com")
	t.Setenv("PORTAL_BOOTSTRAP_ADMIN_PASSWORD", "owner-password")
	t.Setenv("SUB2API_DEFAULT_KEY_QUOTA", "123.5")

	cfg, err := LoadConfigFromEnv()
	if err != nil {
		t.Fatalf("LoadConfigFromEnv returned error: %v", err)
	}
	if cfg.Sub2API.AdminEmail != "admin@example.com" {
		t.Fatalf("admin email = %q, want normalized email", cfg.Sub2API.AdminEmail)
	}
	if cfg.BootstrapAdmin.Email != "owner@example.com" {
		t.Fatalf("bootstrap email = %q, want normalized email", cfg.BootstrapAdmin.Email)
	}
	if cfg.Sub2API.DefaultKeyQuota != 123.5 {
		t.Fatalf("default key quota = %v, want 123.5", cfg.Sub2API.DefaultKeyQuota)
	}
	if cfg.PublicSub2APIBaseURL != cfg.Sub2API.BaseURL {
		t.Fatalf("public sub2api base url = %q, want %q", cfg.PublicSub2APIBaseURL, cfg.Sub2API.BaseURL)
	}
}

func TestLoadConfigFromEnvRequiresSecrets(t *testing.T) {
	t.Setenv("PORTAL_DATABASE_PASSWORD", "db-password")
	t.Setenv("SUB2API_ADMIN_EMAIL", "admin@example.com")
	t.Setenv("SUB2API_ADMIN_PASSWORD", "admin-password")

	if _, err := LoadConfigFromEnv(); err == nil {
		t.Fatal("expected missing PORTAL_SESSION_SECRET error")
	}
}

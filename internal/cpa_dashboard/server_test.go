package cpadashboard

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestParseQueryFilterDefaultsToSevenDays(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/api/quota-efficiency", nil)
	filter, errFilter := parseQueryFilter(req)
	if errFilter != nil {
		t.Fatalf("parse query filter: %v", errFilter)
	}
	if filter.Start.IsZero() || filter.End.IsZero() {
		t.Fatalf("expected default start/end, got %#v", filter)
	}
	if filter.End.Sub(filter.Start).Hours() < 24*6 {
		t.Fatalf("expected roughly seven day range, got %s", filter.End.Sub(filter.Start))
	}
}

func TestParseQueryFilterRejectsLongRange(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/api/quota-efficiency?start=2026-01-01&end=2026-06-01", nil)
	_, errFilter := parseQueryFilter(req)
	if errFilter == nil || !strings.Contains(errFilter.Error(), "90 days") {
		t.Fatalf("expected max range error, got %v", errFilter)
	}
}

func TestSessionRequiresPassword(t *testing.T) {
	server := &Server{cfg: Config{LoginPassword: "secret"}, sessionToken: "session"}
	req := httptest.NewRequest(http.MethodPost, "/api/session", strings.NewReader(`{"password":"wrong"}`))
	rec := httptest.NewRecorder()

	server.handleSession(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusUnauthorized)
	}
}

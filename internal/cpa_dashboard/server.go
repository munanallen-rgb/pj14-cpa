package cpadashboard

import (
	"crypto/rand"
	"crypto/subtle"
	"embed"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io/fs"
	"net/http"
	"strconv"
	"strings"
	"time"

	log "github.com/sirupsen/logrus"
)

const sessionCookieName = "cpa_dashboard_session"

//go:embed static/*
var staticFiles embed.FS

// Server owns the dashboard HTTP routes.
type Server struct {
	cfg          Config
	service      *Service
	sessionToken string
}

// NewServer creates a dashboard HTTP server.
func NewServer(cfg Config, store *Store) *Server {
	return &Server{
		cfg:          cfg,
		service:      NewService(store),
		sessionToken: newSessionToken(),
	}
}

// Handler returns the dashboard HTTP handler.
func (s *Server) Handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("/health", s.handleHealth)
	mux.HandleFunc("/api/session", s.handleSession)
	mux.HandleFunc("/api/overview", s.withAuth(s.handleOverview))
	mux.HandleFunc("/api/quota-efficiency", s.withAuth(s.handleQuotaEfficiency))
	mux.HandleFunc("/api/cpa-accounts", s.withAuth(s.handleCPAAccounts))
	mux.HandleFunc("/api/usage", s.withAuth(s.handleUsage))
	mux.HandleFunc("/api/cleanup-candidates", s.withAuth(s.handleCleanupCandidates))
	mux.HandleFunc("/api/filters", s.withAuth(s.handleFilters))
	mux.HandleFunc("/", s.handleStatic)
	return mux
}

func (s *Server) handleHealth(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (s *Server) handleSession(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		writeJSON(w, http.StatusOK, map[string]bool{"authenticated": s.authenticated(r)})
	case http.MethodPost:
		var req struct {
			Password string `json:"password"`
		}
		if errDecode := json.NewDecoder(r.Body).Decode(&req); errDecode != nil {
			writeError(w, http.StatusBadRequest, "invalid request body")
			return
		}
		if subtle.ConstantTimeCompare([]byte(req.Password), []byte(s.cfg.LoginPassword)) != 1 {
			writeError(w, http.StatusUnauthorized, "invalid password")
			return
		}
		http.SetCookie(w, &http.Cookie{
			Name:     sessionCookieName,
			Value:    s.sessionToken,
			Path:     "/",
			HttpOnly: true,
			SameSite: http.SameSiteLaxMode,
			MaxAge:   int((12 * time.Hour).Seconds()),
		})
		writeJSON(w, http.StatusOK, map[string]bool{"authenticated": true})
	case http.MethodDelete:
		http.SetCookie(w, &http.Cookie{
			Name:     sessionCookieName,
			Value:    "",
			Path:     "/",
			HttpOnly: true,
			SameSite: http.SameSiteLaxMode,
			MaxAge:   -1,
		})
		writeJSON(w, http.StatusOK, map[string]bool{"authenticated": false})
	default:
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
	}
}

func (s *Server) handleOverview(w http.ResponseWriter, r *http.Request) {
	resp, errOverview := s.service.Overview(r.Context())
	if errOverview != nil {
		log.WithError(errOverview).Warn("dashboard overview failed")
		writeError(w, http.StatusInternalServerError, "failed to load overview")
		return
	}
	writeJSON(w, http.StatusOK, resp)
}

func (s *Server) handleQuotaEfficiency(w http.ResponseWriter, r *http.Request) {
	filter, errFilter := parseQueryFilter(r)
	if errFilter != nil {
		writeError(w, http.StatusBadRequest, errFilter.Error())
		return
	}
	resp, errReport := s.service.QuotaEfficiency(r.Context(), filter)
	if errReport != nil {
		log.WithError(errReport).Warn("dashboard quota efficiency failed")
		writeError(w, http.StatusInternalServerError, "failed to load quota efficiency")
		return
	}
	writeJSON(w, http.StatusOK, resp)
}

func (s *Server) handleCPAAccounts(w http.ResponseWriter, r *http.Request) {
	cpa := strings.TrimSpace(r.URL.Query().Get("cpa_source"))
	rows, errRows := s.service.store.CurrentAccounts(r.Context(), cpa)
	if errRows != nil {
		log.WithError(errRows).Warn("dashboard cpa accounts failed")
		writeError(w, http.StatusInternalServerError, "failed to load cpa accounts")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"rows": rows, "generated_at": time.Now().UTC()})
}

func (s *Server) handleUsage(w http.ResponseWriter, r *http.Request) {
	filter, errFilter := parseQueryFilter(r)
	if errFilter != nil {
		writeError(w, http.StatusBadRequest, errFilter.Error())
		return
	}
	buckets, errBuckets := s.service.store.UsageBuckets(r.Context(), filter)
	if errBuckets != nil {
		log.WithError(errBuckets).Warn("dashboard usage failed")
		writeError(w, http.StatusInternalServerError, "failed to load usage")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"filter": filter, "buckets": buckets, "generated_at": time.Now().UTC()})
}

func (s *Server) handleCleanupCandidates(w http.ResponseWriter, r *http.Request) {
	cpa := strings.TrimSpace(r.URL.Query().Get("cpa_source"))
	rows, errRows := s.service.store.CleanupCandidates(r.Context(), cpa)
	if errRows != nil {
		log.WithError(errRows).Warn("dashboard cleanup candidates failed")
		writeError(w, http.StatusInternalServerError, "failed to load cleanup candidates")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"rows": rows, "generated_at": time.Now().UTC()})
}

func (s *Server) handleFilters(w http.ResponseWriter, r *http.Request) {
	resp, errFilters := s.service.store.Filters(r.Context())
	if errFilters != nil {
		log.WithError(errFilters).Warn("dashboard filters failed")
		writeError(w, http.StatusInternalServerError, "failed to load filters")
		return
	}
	writeJSON(w, http.StatusOK, resp)
}

func (s *Server) handleStatic(w http.ResponseWriter, r *http.Request) {
	sub, errSub := fs.Sub(staticFiles, "static")
	if errSub != nil {
		writeError(w, http.StatusInternalServerError, "static files unavailable")
		return
	}
	path := strings.TrimPrefix(r.URL.Path, "/")
	if path == "" {
		path = "index.html"
	}
	file, errOpen := sub.Open(path)
	if errOpen != nil {
		path = "index.html"
	} else if errClose := file.Close(); errClose != nil {
		log.WithError(errClose).Debug("dashboard static file close failed")
	}
	http.ServeFileFS(w, r, sub, path)
}

func (s *Server) withAuth(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if !s.authenticated(r) {
			writeError(w, http.StatusUnauthorized, "authentication required")
			return
		}
		next(w, r)
	}
}

func (s *Server) authenticated(r *http.Request) bool {
	cookie, errCookie := r.Cookie(sessionCookieName)
	if errCookie != nil {
		return false
	}
	return subtle.ConstantTimeCompare([]byte(cookie.Value), []byte(s.sessionToken)) == 1
}

func parseQueryFilter(r *http.Request) (QueryFilter, error) {
	now := time.Now()
	query := r.URL.Query()
	start, errStart := parseTimeParam(query.Get("start"))
	if errStart != nil {
		return QueryFilter{}, fmt.Errorf("invalid start")
	}
	end, errEnd := parseTimeParam(query.Get("end"))
	if errEnd != nil {
		return QueryFilter{}, fmt.Errorf("invalid end")
	}
	if start.IsZero() {
		start = now.AddDate(0, 0, -defaultRangeDays)
	}
	if end.IsZero() {
		end = now
	}
	if !start.Before(end) {
		return QueryFilter{}, fmt.Errorf("start must be before end")
	}
	if end.Sub(start) > time.Duration(maxRangeDays)*24*time.Hour {
		return QueryFilter{}, fmt.Errorf("time range must be %d days or less", maxRangeDays)
	}
	var apiKeyID int64
	if rawAPIKey := strings.TrimSpace(query.Get("api_key_id")); rawAPIKey != "" {
		parsed, errParse := strconv.ParseInt(rawAPIKey, 10, 64)
		if errParse != nil || parsed < 0 {
			return QueryFilter{}, fmt.Errorf("invalid api_key_id")
		}
		apiKeyID = parsed
	}
	return QueryFilter{
		Start:    start,
		End:      end,
		CPA:      strings.TrimSpace(query.Get("cpa_source")),
		Model:    strings.TrimSpace(query.Get("model")),
		APIKeyID: apiKeyID,
	}, nil
}

func parseTimeParam(raw string) (time.Time, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return time.Time{}, nil
	}
	if parsed, errParse := time.Parse(time.RFC3339, raw); errParse == nil {
		return parsed, nil
	}
	if parsed, errParse := time.Parse("2006-01-02", raw); errParse == nil {
		return parsed, nil
	}
	return time.Time{}, fmt.Errorf("invalid time")
}

func writeJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if errEncode := json.NewEncoder(w).Encode(value); errEncode != nil {
		log.WithError(errEncode).Warn("dashboard json response failed")
	}
}

func writeError(w http.ResponseWriter, status int, message string) {
	writeJSON(w, status, map[string]string{"error": message})
}

func newSessionToken() string {
	buf := make([]byte, 32)
	if _, errRead := rand.Read(buf); errRead != nil {
		return strconv.FormatInt(time.Now().UnixNano(), 36)
	}
	return hex.EncodeToString(buf)
}

package portal

import (
	"embed"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"net/http"
	"strconv"
	"strings"
	"time"

	log "github.com/sirupsen/logrus"
)

const sessionCookieName = "portal_session"

//go:embed static/*
var portalStaticFiles embed.FS

// Server owns Portal HTTP routes.
type Server struct {
	cfg     Config
	service *Service
}

// NewServer creates a Portal HTTP server.
func NewServer(cfg Config, service *Service) *Server {
	return &Server{cfg: cfg, service: service}
}

// Handler returns the Portal HTTP handler.
func (s *Server) Handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("/health", s.handleHealth)
	mux.HandleFunc("/api/auth/register", s.handleRegister)
	mux.HandleFunc("/api/auth/login", s.handleLogin)
	mux.HandleFunc("/api/auth/logout", s.withUser(s.handleLogout))
	mux.HandleFunc("/api/me", s.withUser(s.handleMe))
	mux.HandleFunc("/api/api-keys", s.withUser(s.handleAPIKeys))
	mux.HandleFunc("/api/usage/summary", s.withUser(s.handleUsageSummary))
	mux.HandleFunc("/api/usage/records", s.withUser(s.handleUsageRecords))
	mux.HandleFunc("/api/recharge-orders", s.withUser(s.handleRechargeOrders))
	mux.HandleFunc("/api/billing/ledger", s.withUser(s.handleLedger))
	mux.HandleFunc("/api/admin/recharge-orders", s.withAdmin(s.handleAdminRechargeOrders))
	mux.HandleFunc("/api/admin/recharge-orders/", s.withAdmin(s.handleAdminRechargeOrderAction))
	mux.HandleFunc("/", s.handleStatic)
	return s.withCORS(mux)
}

func (s *Server) handleHealth(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (s *Server) handleRegister(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	var req struct {
		Email    string `json:"email"`
		Password string `json:"password"`
	}
	if !decodeJSON(w, r, &req) {
		return
	}
	user, token, expiresAt, errRegister := s.service.Register(r.Context(), req.Email, req.Password)
	if errRegister != nil {
		s.writeServiceError(w, errRegister, "registration failed")
		return
	}
	s.setSessionCookie(w, token, expiresAt)
	writeJSON(w, http.StatusOK, map[string]any{"user": user})
}

func (s *Server) handleLogin(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	var req struct {
		Email    string `json:"email"`
		Password string `json:"password"`
	}
	if !decodeJSON(w, r, &req) {
		return
	}
	user, token, expiresAt, errLogin := s.service.Login(r.Context(), req.Email, req.Password)
	if errLogin != nil {
		s.writeServiceError(w, errLogin, "login failed")
		return
	}
	s.setSessionCookie(w, token, expiresAt)
	writeJSON(w, http.StatusOK, map[string]any{"user": user})
}

func (s *Server) handleLogout(w http.ResponseWriter, r *http.Request, _ AuthContext) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	token := sessionTokenFromRequest(r)
	if errLogout := s.service.Logout(r.Context(), token); errLogout != nil {
		s.writeServiceError(w, errLogout, "logout failed")
		return
	}
	s.clearSessionCookie(w)
	writeJSON(w, http.StatusOK, map[string]bool{"authenticated": false})
}

func (s *Server) handleMe(w http.ResponseWriter, _ *http.Request, auth AuthContext) {
	writeJSON(w, http.StatusOK, map[string]any{
		"user":             auth.User,
		"sub2api_base_url": strings.TrimRight(s.cfg.PublicSub2APIBaseURL, "/") + "/v1",
	})
}

func (s *Server) handleAPIKeys(w http.ResponseWriter, r *http.Request, auth AuthContext) {
	switch r.Method {
	case http.MethodGet:
		items, errList := s.service.ListAPIKeys(r.Context(), auth.User)
		if errList != nil {
			s.writeServiceError(w, errList, "failed to list api keys")
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"items": items})
	case http.MethodPost:
		var req struct {
			Name string `json:"name"`
		}
		if !decodeJSON(w, r, &req) {
			return
		}
		item, errCreate := s.service.CreateAPIKey(r.Context(), auth.User, req.Name)
		if errCreate != nil {
			s.writeServiceError(w, errCreate, "failed to create api key")
			return
		}
		writeJSON(w, http.StatusOK, item)
	default:
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
	}
}

func (s *Server) handleUsageSummary(w http.ResponseWriter, r *http.Request, auth AuthContext) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	filter, errFilter := parseUsageFilter(r)
	if errFilter != nil {
		writeError(w, http.StatusBadRequest, errFilter.Error())
		return
	}
	resp, errSummary := s.service.UsageSummary(r.Context(), auth.User, filter)
	if errSummary != nil {
		s.writeServiceError(w, errSummary, "failed to load usage summary")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"summary": resp, "filter": filter})
}

func (s *Server) handleUsageRecords(w http.ResponseWriter, r *http.Request, auth AuthContext) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	filter, errFilter := parseUsageFilter(r)
	if errFilter != nil {
		writeError(w, http.StatusBadRequest, errFilter.Error())
		return
	}
	items, errRecords := s.service.UsageRecords(r.Context(), auth.User, filter)
	if errRecords != nil {
		s.writeServiceError(w, errRecords, "failed to load usage records")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": items, "filter": filter})
}

func (s *Server) handleRechargeOrders(w http.ResponseWriter, r *http.Request, auth AuthContext) {
	switch r.Method {
	case http.MethodGet:
		items, errList := s.service.ListRechargeOrders(r.Context(), auth.User, r.URL.Query().Get("status"))
		if errList != nil {
			s.writeServiceError(w, errList, "failed to list recharge orders")
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"items": items})
	case http.MethodPost:
		var req struct {
			Amount   float64 `json:"amount"`
			Currency string  `json:"currency"`
			Note     string  `json:"note"`
		}
		if !decodeJSON(w, r, &req) {
			return
		}
		order, errCreate := s.service.CreateRechargeOrder(r.Context(), auth.User, req.Amount, req.Currency, req.Note)
		if errCreate != nil {
			s.writeServiceError(w, errCreate, "failed to create recharge order")
			return
		}
		writeJSON(w, http.StatusOK, order)
	default:
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
	}
}

func (s *Server) handleLedger(w http.ResponseWriter, r *http.Request, auth AuthContext) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	items, errLedger := s.service.ListLedgerEntries(r.Context(), auth.User)
	if errLedger != nil {
		s.writeServiceError(w, errLedger, "failed to list ledger")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": items})
}

func (s *Server) handleAdminRechargeOrders(w http.ResponseWriter, r *http.Request, auth AuthContext) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	items, errList := s.service.AdminListRechargeOrders(r.Context(), auth.User, r.URL.Query().Get("status"))
	if errList != nil {
		s.writeServiceError(w, errList, "failed to list admin recharge orders")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": items})
}

func (s *Server) handleAdminRechargeOrderAction(w http.ResponseWriter, r *http.Request, auth AuthContext) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	orderID, action, errParse := parseAdminRechargeAction(r.URL.Path)
	if errParse != nil {
		writeError(w, http.StatusNotFound, "not found")
		return
	}
	switch action {
	case "confirm":
		ledger, errConfirm := s.service.AdminConfirmRechargeOrder(r.Context(), auth.User, orderID)
		if errConfirm != nil {
			s.writeServiceError(w, errConfirm, "failed to confirm recharge order")
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"ledger_entry": ledger})
	case "cancel":
		if errCancel := s.service.AdminCancelRechargeOrder(r.Context(), auth.User, orderID); errCancel != nil {
			s.writeServiceError(w, errCancel, "failed to cancel recharge order")
			return
		}
		writeJSON(w, http.StatusOK, map[string]string{"status": statusCancelled})
	default:
		writeError(w, http.StatusNotFound, "not found")
	}
}

func (s *Server) handleStatic(w http.ResponseWriter, r *http.Request) {
	sub, errSub := fs.Sub(portalStaticFiles, "static")
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
		log.WithError(errClose).Debug("portal static file close failed")
	}
	http.ServeFileFS(w, r, sub, path)
}

type authedHandler func(http.ResponseWriter, *http.Request, AuthContext)

func (s *Server) withUser(next authedHandler) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		user, errUser := s.service.UserBySession(r.Context(), sessionTokenFromRequest(r))
		if errUser != nil {
			writeError(w, http.StatusUnauthorized, "authentication required")
			return
		}
		next(w, r, AuthContext{User: user})
	}
}

func (s *Server) withAdmin(next authedHandler) http.HandlerFunc {
	return s.withUser(func(w http.ResponseWriter, r *http.Request, auth AuthContext) {
		if auth.User.Role != roleAdmin {
			writeError(w, http.StatusForbidden, "admin role required")
			return
		}
		next(w, r, auth)
	})
}

func (s *Server) withCORS(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		origin := strings.TrimSpace(r.Header.Get("Origin"))
		if origin != "" && s.originAllowed(origin) {
			w.Header().Set("Access-Control-Allow-Origin", origin)
			w.Header().Set("Access-Control-Allow-Credentials", "true")
			w.Header().Set("Access-Control-Allow-Headers", "Content-Type")
			w.Header().Set("Access-Control-Allow-Methods", "GET,POST,PATCH,DELETE,OPTIONS")
		}
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		next.ServeHTTP(w, r)
	})
}

func (s *Server) originAllowed(origin string) bool {
	if len(s.cfg.AllowedOrigins) == 0 {
		return false
	}
	for _, allowed := range s.cfg.AllowedOrigins {
		if allowed == "*" || strings.EqualFold(allowed, origin) {
			return true
		}
	}
	return false
}

func (s *Server) setSessionCookie(w http.ResponseWriter, token string, expiresAt time.Time) {
	http.SetCookie(w, &http.Cookie{
		Name:     sessionCookieName,
		Value:    token,
		Path:     "/",
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
		Secure:   s.cfg.CookieSecure,
		Expires:  expiresAt,
		MaxAge:   int(time.Until(expiresAt).Seconds()),
	})
}

func (s *Server) clearSessionCookie(w http.ResponseWriter) {
	http.SetCookie(w, &http.Cookie{
		Name:     sessionCookieName,
		Value:    "",
		Path:     "/",
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
		Secure:   s.cfg.CookieSecure,
		MaxAge:   -1,
	})
}

func sessionTokenFromRequest(r *http.Request) string {
	cookie, errCookie := r.Cookie(sessionCookieName)
	if errCookie != nil {
		return ""
	}
	return cookie.Value
}

func parseUsageFilter(r *http.Request) (UsageFilter, error) {
	query := r.URL.Query()
	start, errStart := parseTimeParam(query.Get("start"))
	if errStart != nil {
		return UsageFilter{}, fmt.Errorf("invalid start")
	}
	end, errEnd := parseTimeParam(query.Get("end"))
	if errEnd != nil {
		return UsageFilter{}, fmt.Errorf("invalid end")
	}
	apiKeyID, errAPIKey := parseOptionalInt64(query.Get("api_key_id"))
	if errAPIKey != nil {
		return UsageFilter{}, fmt.Errorf("invalid api_key_id")
	}
	limit := defaultUsageRecordLimit
	if rawLimit := strings.TrimSpace(query.Get("limit")); rawLimit != "" {
		parsed, errLimit := strconv.Atoi(rawLimit)
		if errLimit != nil || parsed <= 0 {
			return UsageFilter{}, fmt.Errorf("invalid limit")
		}
		limit = parsed
	}
	return UsageFilter{Start: start, End: end, APIKeyID: apiKeyID, Limit: limit}, nil
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

func parseOptionalInt64(raw string) (int64, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return 0, nil
	}
	parsed, errParse := strconv.ParseInt(raw, 10, 64)
	if errParse != nil || parsed < 0 {
		return 0, fmt.Errorf("invalid integer")
	}
	return parsed, nil
}

func parseAdminRechargeAction(path string) (int64, string, error) {
	rest := strings.TrimPrefix(path, "/api/admin/recharge-orders/")
	parts := strings.Split(strings.Trim(rest, "/"), "/")
	if len(parts) != 2 {
		return 0, "", fmt.Errorf("invalid path")
	}
	id, errID := strconv.ParseInt(parts[0], 10, 64)
	if errID != nil || id <= 0 {
		return 0, "", fmt.Errorf("invalid id")
	}
	return id, parts[1], nil
}

func decodeJSON(w http.ResponseWriter, r *http.Request, out any) bool {
	if r.Body == nil {
		writeError(w, http.StatusBadRequest, "request body required")
		return false
	}
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()
	if errDecode := decoder.Decode(out); errDecode != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return false
	}
	return true
}

func writeJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if errEncode := json.NewEncoder(w).Encode(value); errEncode != nil {
		log.WithError(errEncode).Warn("portal json response failed")
	}
}

func writeError(w http.ResponseWriter, status int, message string) {
	writeJSON(w, status, map[string]string{"error": message})
}

func (s *Server) writeServiceError(w http.ResponseWriter, err error, fallback string) {
	switch {
	case errors.Is(err, ErrAlreadyExists):
		writeError(w, http.StatusConflict, "already exists")
	case errors.Is(err, ErrInvalidInput):
		writeError(w, http.StatusBadRequest, strings.TrimPrefix(err.Error(), ErrInvalidInput.Error()+": "))
	case errors.Is(err, ErrNotFound):
		writeError(w, http.StatusNotFound, "not found")
	case errors.Is(err, ErrForbidden):
		writeError(w, http.StatusForbidden, "forbidden")
	default:
		log.WithError(err).Warn(fallback)
		writeError(w, http.StatusInternalServerError, fallback)
	}
}

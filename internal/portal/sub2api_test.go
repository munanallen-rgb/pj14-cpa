package portal

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestSub2APIClientEnsureUserAndCreateAPIKey(t *testing.T) {
	var createdUserBody map[string]any
	var createdKeyBody map[string]any
	var groupCreated bool
	var userCreated bool

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch {
		case r.Method == http.MethodPost && r.URL.Path == "/api/v1/auth/login":
			var body map[string]string
			if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
				t.Fatalf("decode login body: %v", err)
			}
			if body["email"] == "admin@example.com" {
				_, _ = w.Write([]byte(`{"data":{"access_token":"admin-token"}}`))
				return
			}
			if body["email"] == "user@example.com" && body["password"] == "sub2api-password" {
				_, _ = w.Write([]byte(`{"data":{"access_token":"user-token"}}`))
				return
			}
			http.Error(w, "bad login", http.StatusUnauthorized)
		case r.Method == http.MethodGet && r.URL.Path == "/api/v1/admin/groups":
			if r.Header.Get("Authorization") != "Bearer admin-token" {
				t.Fatalf("group list authorization = %q", r.Header.Get("Authorization"))
			}
			if groupCreated {
				_, _ = w.Write([]byte(`{"data":{"items":[{"id":7,"name":"cpa-openai"}]}}`))
				return
			}
			_, _ = w.Write([]byte(`{"data":{"items":[]}}`))
		case r.Method == http.MethodPost && r.URL.Path == "/api/v1/admin/groups":
			groupCreated = true
			_, _ = w.Write([]byte(`{"data":{"id":7,"name":"cpa-openai"}}`))
		case r.Method == http.MethodGet && r.URL.Path == "/api/v1/admin/users":
			if userCreated {
				_, _ = w.Write([]byte(`{"data":{"items":[{"id":42,"email":"user@example.com","balance":0}]}}`))
				return
			}
			_, _ = w.Write([]byte(`{"data":{"items":[]}}`))
		case r.Method == http.MethodPost && r.URL.Path == "/api/v1/admin/users":
			userCreated = true
			if err := json.NewDecoder(r.Body).Decode(&createdUserBody); err != nil {
				t.Fatalf("decode create user body: %v", err)
			}
			_, _ = w.Write([]byte(`{"data":{"id":42,"email":"user@example.com","balance":0}}`))
		case r.Method == http.MethodPost && r.URL.Path == "/api/v1/keys":
			if r.Header.Get("Authorization") != "Bearer user-token" {
				t.Fatalf("key create authorization = %q", r.Header.Get("Authorization"))
			}
			if err := json.NewDecoder(r.Body).Decode(&createdKeyBody); err != nil {
				t.Fatalf("decode create key body: %v", err)
			}
			_, _ = w.Write([]byte(`{"data":{"id":99,"key":"sk-created","name":"Portal key","group_id":7,"status":"active"}}`))
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	client := NewSub2APIClient(Sub2APIConfig{
		BaseURL:         server.URL,
		AdminEmail:      "admin@example.com",
		AdminPassword:   "admin-password",
		DefaultGroup:    "cpa-openai",
		DefaultKeyQuota: 10,
	})

	user, errUser := client.EnsureUser(t.Context(), "user@example.com", "sub2api-password")
	if errUser != nil {
		t.Fatalf("EnsureUser returned error: %v", errUser)
	}
	if user.ID != 42 {
		t.Fatalf("user.ID = %d, want 42", user.ID)
	}
	if createdUserBody["email"] != "user@example.com" {
		t.Fatalf("created user email = %v", createdUserBody["email"])
	}
	allowedGroups, ok := createdUserBody["allowed_groups"].([]any)
	if !ok || len(allowedGroups) != 1 || allowedGroups[0].(float64) != 7 {
		t.Fatalf("allowed_groups = %#v, want [7]", createdUserBody["allowed_groups"])
	}

	key, errKey := client.CreateAPIKey(t.Context(), "user@example.com", "sub2api-password", "Portal key")
	if errKey != nil {
		t.Fatalf("CreateAPIKey returned error: %v", errKey)
	}
	if key.ID != 99 || key.Key != "sk-created" {
		t.Fatalf("key = %#v, want id 99 and sk-created", key)
	}
	if createdKeyBody["group_id"].(float64) != 7 {
		t.Fatalf("created key group_id = %v, want 7", createdKeyBody["group_id"])
	}
	if createdKeyBody["quota"].(float64) != 10 {
		t.Fatalf("created key quota = %v, want 10", createdKeyBody["quota"])
	}
}

func TestSub2APIClientAddBalance(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if r.Method == http.MethodPost && r.URL.Path == "/api/v1/auth/login" {
			_, _ = w.Write([]byte(`{"data":{"access_token":"admin-token"}}`))
			return
		}
		if r.Method == http.MethodPost && r.URL.Path == "/api/v1/admin/users/42/balance" {
			if r.Header.Get("Authorization") != "Bearer admin-token" {
				t.Fatalf("balance authorization = %q", r.Header.Get("Authorization"))
			}
			var body map[string]any
			if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
				t.Fatalf("decode balance body: %v", err)
			}
			if body["operation"] != "add" {
				t.Fatalf("operation = %v, want add", body["operation"])
			}
			_, _ = w.Write([]byte(`{"data":{"id":42,"email":"user@example.com","balance":25.5}}`))
			return
		}
		http.NotFound(w, r)
	}))
	defer server.Close()

	client := NewSub2APIClient(Sub2APIConfig{
		BaseURL:       server.URL,
		AdminEmail:    "admin@example.com",
		AdminPassword: "admin-password",
		DefaultGroup:  "cpa-openai",
	})

	user, err := client.AddBalance(t.Context(), 42, 25.5, "manual")
	if err != nil {
		t.Fatalf("AddBalance returned error: %v", err)
	}
	if user.Balance != 25.5 {
		t.Fatalf("balance = %v, want 25.5", user.Balance)
	}
}

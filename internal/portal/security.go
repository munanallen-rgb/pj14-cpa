package portal

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"net/mail"
	"strings"

	"golang.org/x/crypto/bcrypt"
)

const minPasswordLength = 8

func normalizeEmail(value string) string {
	return strings.ToLower(strings.TrimSpace(value))
}

func validateEmail(value string) error {
	if value == "" {
		return fmt.Errorf("email is required")
	}
	parsed, errParse := mail.ParseAddress(value)
	if errParse != nil || normalizeEmail(parsed.Address) != value {
		return fmt.Errorf("email is invalid")
	}
	return nil
}

func validatePassword(value string) error {
	if len(value) < minPasswordLength {
		return fmt.Errorf("password must be at least %d characters", minPasswordLength)
	}
	return nil
}

func hashPassword(password string) (string, error) {
	hashed, errHash := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if errHash != nil {
		return "", fmt.Errorf("hash password: %w", errHash)
	}
	return string(hashed), nil
}

func verifyPassword(hash string, password string) bool {
	return bcrypt.CompareHashAndPassword([]byte(hash), []byte(password)) == nil
}

func newSessionToken() (string, string, error) {
	buf := make([]byte, 32)
	if _, errRead := rand.Read(buf); errRead != nil {
		return "", "", fmt.Errorf("generate session token: %w", errRead)
	}
	token := base64.RawURLEncoding.EncodeToString(buf)
	return token, hashSessionToken(token), nil
}

func hashSessionToken(token string) string {
	sum := sha256.Sum256([]byte(token))
	return hex.EncodeToString(sum[:])
}

func deriveSub2APIPassword(secret string, userID int64) string {
	mac := hmac.New(sha256.New, []byte(secret))
	_, _ = fmt.Fprintf(mac, "sub2api-user:%d", userID)
	return base64.RawURLEncoding.EncodeToString(mac.Sum(nil))
}

func keyPreview(key string) string {
	key = strings.TrimSpace(key)
	if key == "" {
		return ""
	}
	if len(key) <= 12 {
		return key[:minInt(4, len(key))] + "..."
	}
	return key[:6] + "..." + key[len(key)-4:]
}

func minInt(a int, b int) int {
	if a < b {
		return a
	}
	return b
}

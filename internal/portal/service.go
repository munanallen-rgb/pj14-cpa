package portal

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"
)

// Service coordinates Portal business operations.
type Service struct {
	cfg     Config
	store   *Store
	sub2api Sub2API
	now     func() time.Time
}

// NewService creates a Portal service.
func NewService(cfg Config, store *Store, sub2api Sub2API) *Service {
	return &Service{
		cfg:     cfg,
		store:   store,
		sub2api: sub2api,
		now:     time.Now,
	}
}

// BootstrapAdmin creates the first admin when bootstrap credentials are configured.
func (s *Service) BootstrapAdmin(ctx context.Context) error {
	if s.cfg.BootstrapAdmin.Email == "" {
		return nil
	}
	if _, errExisting := s.store.GetUserByEmail(ctx, s.cfg.BootstrapAdmin.Email); errExisting == nil {
		return nil
	} else if !errors.Is(errExisting, ErrNotFound) {
		return errExisting
	}
	if errEmail := validateEmail(s.cfg.BootstrapAdmin.Email); errEmail != nil {
		return errEmail
	}
	if errPassword := validatePassword(s.cfg.BootstrapAdmin.Password); errPassword != nil {
		return errPassword
	}
	passwordHash, errHash := hashPassword(s.cfg.BootstrapAdmin.Password)
	if errHash != nil {
		return errHash
	}
	if _, errCreate := s.store.CreateUser(ctx, s.cfg.BootstrapAdmin.Email, passwordHash, roleAdmin); errCreate != nil {
		return errCreate
	}
	return nil
}

func (s *Service) Register(ctx context.Context, email string, password string) (User, string, time.Time, error) {
	email = normalizeEmail(email)
	if errEmail := validateEmail(email); errEmail != nil {
		return User{}, "", time.Time{}, errEmail
	}
	if errPassword := validatePassword(password); errPassword != nil {
		return User{}, "", time.Time{}, errPassword
	}
	passwordHash, errHash := hashPassword(password)
	if errHash != nil {
		return User{}, "", time.Time{}, errHash
	}
	user, errCreate := s.store.CreateUser(ctx, email, passwordHash, roleUser)
	if errCreate != nil {
		return User{}, "", time.Time{}, errCreate
	}
	if _, errEnsure := s.ensureSub2APIUser(ctx, user); errEnsure != nil {
		return User{}, "", time.Time{}, errEnsure
	}
	token, expiresAt, errSession := s.createSession(ctx, user.ID)
	if errSession != nil {
		return User{}, "", time.Time{}, errSession
	}
	return user, token, expiresAt, nil
}

func (s *Service) Login(ctx context.Context, email string, password string) (User, string, time.Time, error) {
	email = normalizeEmail(email)
	user, errUser := s.store.GetUserByEmail(ctx, email)
	if errUser != nil {
		return User{}, "", time.Time{}, ErrForbidden
	}
	if user.Status != statusActive || !verifyPassword(user.PasswordHash, password) {
		return User{}, "", time.Time{}, ErrForbidden
	}
	if _, errEnsure := s.ensureSub2APIUser(ctx, user); errEnsure != nil {
		return User{}, "", time.Time{}, errEnsure
	}
	token, expiresAt, errSession := s.createSession(ctx, user.ID)
	if errSession != nil {
		return User{}, "", time.Time{}, errSession
	}
	return user, token, expiresAt, nil
}

func (s *Service) Logout(ctx context.Context, token string) error {
	if strings.TrimSpace(token) == "" {
		return nil
	}
	return s.store.DeleteSession(ctx, hashSessionToken(token))
}

func (s *Service) UserBySession(ctx context.Context, token string) (User, error) {
	if strings.TrimSpace(token) == "" {
		return User{}, ErrNotFound
	}
	return s.store.UserBySessionTokenHash(ctx, hashSessionToken(token), s.now())
}

func (s *Service) ListAPIKeys(ctx context.Context, user User) ([]APIKey, error) {
	return s.store.ListAPIKeys(ctx, user.ID)
}

func (s *Service) CreateAPIKey(ctx context.Context, user User, name string) (APIKeyCreateResult, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		name = "Default key"
	}
	if len(name) > 80 {
		return APIKeyCreateResult{}, fmt.Errorf("%w: name is too long", ErrInvalidInput)
	}
	mapping, errEnsure := s.ensureSub2APIUser(ctx, user)
	if errEnsure != nil {
		return APIKeyCreateResult{}, errEnsure
	}
	password := deriveSub2APIPassword(s.cfg.SessionSecret, user.ID)
	key, errKey := s.sub2api.CreateAPIKey(ctx, mapping.Sub2APIEmail, password, name)
	if errKey != nil {
		return APIKeyCreateResult{}, errKey
	}
	item, errStore := s.store.CreateAPIKey(ctx, APIKey{
		PortalUserID: user.ID,
		Sub2APIKeyID: key.ID,
		Name:         key.Name,
		KeyPreview:   keyPreview(key.Key),
		GroupName:    s.cfg.Sub2API.DefaultGroup,
		Status:       statusActive,
	})
	if errStore != nil {
		return APIKeyCreateResult{}, errStore
	}
	return APIKeyCreateResult{APIKey: item, Key: key.Key}, nil
}

func (s *Service) UsageSummary(ctx context.Context, user User, filter UsageFilter) (UsageSummary, error) {
	filter = normalizeUsageFilter(filter, s.now())
	return s.store.UsageSummary(ctx, user.ID, filter)
}

func (s *Service) UsageRecords(ctx context.Context, user User, filter UsageFilter) ([]UsageRecord, error) {
	filter = normalizeUsageFilter(filter, s.now())
	return s.store.UsageRecords(ctx, user.ID, filter)
}

func (s *Service) CreateRechargeOrder(ctx context.Context, user User, amount float64, currency string, note string) (RechargeOrder, error) {
	if amount <= 0 {
		return RechargeOrder{}, fmt.Errorf("%w: amount must be positive", ErrInvalidInput)
	}
	if amount > 1000000 {
		return RechargeOrder{}, fmt.Errorf("%w: amount is too large", ErrInvalidInput)
	}
	return s.store.CreateRechargeOrder(ctx, user.ID, amount, normalizeCurrency(currency), strings.TrimSpace(note))
}

func (s *Service) ListRechargeOrders(ctx context.Context, user User, status string) ([]RechargeOrder, error) {
	return s.store.ListRechargeOrders(ctx, user.ID, strings.TrimSpace(status), false)
}

func (s *Service) ListLedgerEntries(ctx context.Context, user User) ([]LedgerEntry, error) {
	return s.store.ListLedgerEntries(ctx, user.ID)
}

func (s *Service) AdminListRechargeOrders(ctx context.Context, admin User, status string) ([]RechargeOrder, error) {
	if admin.Role != roleAdmin {
		return nil, ErrForbidden
	}
	return s.store.ListRechargeOrders(ctx, 0, strings.TrimSpace(status), true)
}

func (s *Service) AdminConfirmRechargeOrder(ctx context.Context, admin User, orderID int64) (LedgerEntry, error) {
	if admin.Role != roleAdmin {
		return LedgerEntry{}, ErrForbidden
	}
	order, errMark := s.store.MarkRechargeOrderProcessing(ctx, orderID, admin.ID)
	if errMark != nil {
		return LedgerEntry{}, errMark
	}
	mapping, errMapping := s.store.GetSub2APIUserMapping(ctx, order.UserID)
	if errMapping != nil {
		_ = s.store.ResetRechargeOrderPending(ctx, order.ID)
		return LedgerEntry{}, errMapping
	}
	note := fmt.Sprintf("Portal recharge order #%d", order.ID)
	if strings.TrimSpace(order.Note) != "" {
		note += ": " + strings.TrimSpace(order.Note)
	}
	sub2apiUser, errBalance := s.sub2api.AddBalance(ctx, mapping.Sub2APIUserID, order.Amount, note)
	if errBalance != nil {
		_ = s.store.ResetRechargeOrderPending(ctx, order.ID)
		return LedgerEntry{}, errBalance
	}
	return s.store.ConfirmRechargeOrder(ctx, order, admin.ID, sub2apiUser.Balance)
}

func (s *Service) AdminCancelRechargeOrder(ctx context.Context, admin User, orderID int64) error {
	if admin.Role != roleAdmin {
		return ErrForbidden
	}
	return s.store.CancelRechargeOrder(ctx, orderID, admin.ID)
}

func (s *Service) ensureSub2APIUser(ctx context.Context, user User) (Sub2APIUserMapping, error) {
	mapping, errMapping := s.store.GetSub2APIUserMapping(ctx, user.ID)
	if errMapping == nil {
		return mapping, nil
	}
	if !errors.Is(errMapping, ErrNotFound) {
		return Sub2APIUserMapping{}, errMapping
	}
	password := deriveSub2APIPassword(s.cfg.SessionSecret, user.ID)
	sub2apiUser, errEnsure := s.sub2api.EnsureUser(ctx, user.Email, password)
	if errEnsure != nil {
		return Sub2APIUserMapping{}, errEnsure
	}
	mapping = Sub2APIUserMapping{
		PortalUserID:  user.ID,
		Sub2APIUserID: sub2apiUser.ID,
		Sub2APIEmail:  sub2apiUser.Email,
	}
	if errUpsert := s.store.UpsertSub2APIUserMapping(ctx, mapping); errUpsert != nil {
		return Sub2APIUserMapping{}, errUpsert
	}
	return mapping, nil
}

func (s *Service) createSession(ctx context.Context, userID int64) (string, time.Time, error) {
	token, tokenHash, errToken := newSessionToken()
	if errToken != nil {
		return "", time.Time{}, errToken
	}
	expiresAt := s.now().Add(s.cfg.SessionTTL)
	if _, errSession := s.store.CreateSession(ctx, userID, tokenHash, expiresAt); errSession != nil {
		return "", time.Time{}, errSession
	}
	return token, expiresAt, nil
}

func normalizeUsageFilter(filter UsageFilter, now time.Time) UsageFilter {
	if filter.End.IsZero() {
		filter.End = now
	}
	if filter.Start.IsZero() {
		filter.Start = filter.End.AddDate(0, 0, -7)
	}
	if !filter.Start.Before(filter.End) {
		filter.Start = filter.End.AddDate(0, 0, -7)
	}
	if filter.Limit <= 0 {
		filter.Limit = defaultUsageRecordLimit
	}
	if filter.Limit > defaultMaxUsageRecordLimit {
		filter.Limit = defaultMaxUsageRecordLimit
	}
	return filter
}

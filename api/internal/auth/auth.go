// Package auth: đăng nhập bcrypt trên users (Service quản lý), JWT ngắn hạn, refresh token xoay vòng,
// khoá đăng nhập sau N lần sai (đếm trong portal_login_attempts — Portal không ghi vào users).
package auth

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"strings"
	"time"

	"golang.org/x/crypto/bcrypt"

	"am-shortlink-portal/api/internal/domain"
)

// UserRecord: các trường users mà Portal đọc (hợp đồng schema với Service — PLAN_SERVICE_API.md §5.6).
type UserRecord struct {
	Username       string
	Email          string
	PasswordHash   string
	Role           string
	Active         bool
	PortalAccess   bool
	ViewerAccounts []string
}

type RefreshRecord struct {
	Hash      string
	FamilyID  string
	Username  string
	CreatedAt time.Time
	ExpiresAt time.Time
	RevokedAt *time.Time
	// FamilyRevoked: cả chuỗi bị thu hồi (đăng xuất / phát hiện dùng lại) — không áp dụng grace.
	FamilyRevoked bool
	UserAgent     string
	IP            string
}

type Store interface {
	FindUser(ctx context.Context, username string) (*UserRecord, error) // không có → nil, nil
	LockedUntil(ctx context.Context, username string) (time.Time, error)
	RecordFailure(ctx context.Context, username string, now time.Time, max int, lockFor time.Duration) (lockedUntil time.Time, err error)
	ResetFailures(ctx context.Context, username string) error
	InsertRefresh(ctx context.Context, r RefreshRecord) error
	FindRefresh(ctx context.Context, hash string) (*RefreshRecord, error) // không có → nil, nil
	// RevokeRefresh thu hồi nếu chưa thu hồi; trả false nếu đã bị thu hồi trước đó.
	RevokeRefresh(ctx context.Context, hash string, now time.Time) (bool, error)
	// RevokeFamily đánh dấu cả chuỗi (kể cả token đã xoay) là bị thu hồi.
	RevokeFamily(ctx context.Context, familyID string, now time.Time) error
}

type Options struct {
	MaxFailedLogins int
	LockDuration    time.Duration
	RefreshTTL      time.Duration
	// ReuseGrace: refresh token vừa xoay trong khoảng này vẫn được chấp nhận (request song song của BFF).
	ReuseGrace time.Duration
}

type Service struct {
	store  Store
	tokens *Tokens
	opts   Options
	now    func() time.Time
}

func NewService(store Store, tokens *Tokens, opts Options) *Service {
	if opts.ReuseGrace == 0 {
		opts.ReuseGrace = 10 * time.Second
	}
	return &Service{store: store, tokens: tokens, opts: opts, now: time.Now}
}

// Session là kết quả đăng nhập / refresh.
type Session struct {
	AccessToken      string
	AccessExpiresAt  time.Time
	RefreshToken     string
	RefreshExpiresAt time.Time
	Principal        domain.Principal
}

type ClientInfo struct {
	IP        string
	UserAgent string
}

var (
	errInvalidCredentials = domain.Unauthorized("invalid_credentials", "sai tên đăng nhập hoặc mật khẩu")
	errInvalidRefresh     = domain.Unauthorized("invalid_refresh_token", "phiên đăng nhập không hợp lệ hoặc đã hết hạn")
)

// dummyHash dùng để so bcrypt khi user không tồn tại → thời gian phản hồi như nhau.
var dummyHash, _ = bcrypt.GenerateFromPassword([]byte("dummy-password-for-timing"), bcrypt.DefaultCost)

func (s *Service) Login(ctx context.Context, username, password string, ci ClientInfo) (*Session, error) {
	username = strings.TrimSpace(username)
	now := s.now()

	until, err := s.store.LockedUntil(ctx, username)
	if err != nil {
		return nil, err
	}
	if until.After(now) {
		return nil, lockedErr(until, now)
	}

	u, err := s.store.FindUser(ctx, username)
	if err != nil {
		return nil, err
	}
	hash := dummyHash
	if u != nil {
		hash = []byte(normalizeBcrypt(u.PasswordHash))
	}
	pwOK := bcrypt.CompareHashAndPassword(hash, []byte(password)) == nil

	if u == nil || !pwOK {
		lockedUntil, err := s.store.RecordFailure(ctx, username, now, s.opts.MaxFailedLogins, s.opts.LockDuration)
		if err != nil {
			return nil, err
		}
		if lockedUntil.After(now) {
			return nil, lockedErr(lockedUntil, now)
		}
		return nil, errInvalidCredentials
	}
	if !u.Active || !u.PortalAccess {
		// Mật khẩu đúng nhưng không được vào Portal: báo rõ (không lộ thêm gì vì đã chứng minh biết mật khẩu).
		return nil, domain.Forbidden("portal_access_denied", "tài khoản chưa được cấp quyền vào Portal hoặc đã bị khoá")
	}
	if err := s.store.ResetFailures(ctx, username); err != nil {
		return nil, err
	}
	return s.issue(ctx, u, newFamilyID(), ci)
}

func (s *Service) Refresh(ctx context.Context, refreshToken string, ci ClientInfo) (*Session, error) {
	now := s.now()
	h := HashToken(refreshToken)
	rec, err := s.store.FindRefresh(ctx, h)
	if err != nil {
		return nil, err
	}
	if rec == nil || rec.FamilyRevoked || !rec.ExpiresAt.After(now) {
		return nil, errInvalidRefresh
	}
	if rec.RevokedAt != nil && now.Sub(*rec.RevokedAt) > s.opts.ReuseGrace {
		// Token đã xoay từ lâu bị dùng lại → nghi bị đánh cắp: thu hồi cả chuỗi.
		if err := s.store.RevokeFamily(ctx, rec.FamilyID, now); err != nil {
			return nil, err
		}
		return nil, errInvalidRefresh
	}
	if rec.RevokedAt == nil {
		if _, err := s.store.RevokeRefresh(ctx, h, now); err != nil {
			return nil, err
		}
	}

	// Đọc lại users để khoá / đổi quyền có hiệu lực ngay.
	u, err := s.store.FindUser(ctx, rec.Username)
	if err != nil {
		return nil, err
	}
	if u == nil || !u.Active || !u.PortalAccess {
		_ = s.store.RevokeFamily(ctx, rec.FamilyID, now)
		return nil, errInvalidRefresh
	}
	return s.issue(ctx, u, rec.FamilyID, ci)
}

// Logout thu hồi cả chuỗi refresh token; token không hợp lệ vẫn coi là thành công.
func (s *Service) Logout(ctx context.Context, refreshToken string) error {
	rec, err := s.store.FindRefresh(ctx, HashToken(refreshToken))
	if err != nil || rec == nil {
		return err
	}
	return s.store.RevokeFamily(ctx, rec.FamilyID, s.now())
}

func (s *Service) issue(ctx context.Context, u *UserRecord, family string, ci ClientInfo) (*Session, error) {
	now := s.now()
	p := PrincipalFromUser(u)
	access, accessExp, err := s.tokens.Sign(p, now)
	if err != nil {
		return nil, err
	}
	raw, err := randomToken()
	if err != nil {
		return nil, err
	}
	rec := RefreshRecord{
		Hash:      HashToken(raw),
		FamilyID:  family,
		Username:  u.Username,
		CreatedAt: now,
		ExpiresAt: now.Add(s.opts.RefreshTTL),
		UserAgent: truncate(ci.UserAgent, 256),
		IP:        ci.IP,
	}
	if err := s.store.InsertRefresh(ctx, rec); err != nil {
		return nil, err
	}
	return &Session{
		AccessToken:      access,
		AccessExpiresAt:  accessExp,
		RefreshToken:     raw,
		RefreshExpiresAt: rec.ExpiresAt,
		Principal:        p,
	}, nil
}

func PrincipalFromUser(u *UserRecord) domain.Principal {
	p := domain.Principal{Username: u.Username, Email: u.Email, Role: domain.ParseRole(u.Role)}
	if p.Role == domain.RoleViewer {
		p.ViewerAccounts = u.ViewerAccounts
	}
	return p
}

func lockedErr(until, now time.Time) error {
	mins := int(until.Sub(now).Minutes()) + 1
	return domain.Locked("account_locked", "đăng nhập sai quá nhiều lần, thử lại sau khoảng "+itoa(mins)+" phút")
}

// normalizeBcrypt: hash PHP dạng $2y$ tương đương $2a$ cho thư viện Go.
func normalizeBcrypt(h string) string {
	if strings.HasPrefix(h, "$2y$") {
		return "$2a$" + h[4:]
	}
	return h
}

// HashToken: SHA-256 hex của refresh token (chỉ lưu bản băm).
func HashToken(raw string) string {
	sum := sha256.Sum256([]byte(raw))
	return hex.EncodeToString(sum[:])
}

func randomToken() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return "rt_" + base64.RawURLEncoding.EncodeToString(b), nil
}

func newFamilyID() string {
	b := make([]byte, 16)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n]
}

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	var buf [20]byte
	i := len(buf)
	for n > 0 {
		i--
		buf[i] = byte('0' + n%10)
		n /= 10
	}
	return string(buf[i:])
}

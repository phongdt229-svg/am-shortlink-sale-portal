package auth

import (
	"context"
	"net/http"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"golang.org/x/crypto/bcrypt"

	"am-shortlink-portal/api/internal/domain"
)

type memStore struct {
	mu       sync.Mutex
	users    map[string]*UserRecord
	failures map[string]int
	locked   map[string]time.Time
	refresh  map[string]*RefreshRecord
}

func newMemStore() *memStore {
	return &memStore{
		users: map[string]*UserRecord{}, failures: map[string]int{},
		locked: map[string]time.Time{}, refresh: map[string]*RefreshRecord{},
	}
}

func (m *memStore) FindUser(_ context.Context, u string) (*UserRecord, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if r, ok := m.users[u]; ok {
		c := *r
		return &c, nil
	}
	return nil, nil
}
func (m *memStore) LockedUntil(_ context.Context, u string) (time.Time, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.locked[u], nil
}
func (m *memStore) RecordFailure(_ context.Context, u string, now time.Time, max int, d time.Duration) (time.Time, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.failures[u]++
	if m.failures[u] >= max {
		m.failures[u] = 0
		m.locked[u] = now.Add(d)
	}
	return m.locked[u], nil
}
func (m *memStore) ResetFailures(_ context.Context, u string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.failures[u] = 0
	return nil
}
func (m *memStore) InsertRefresh(_ context.Context, r RefreshRecord) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.refresh[r.Hash] = &r
	return nil
}
func (m *memStore) FindRefresh(_ context.Context, h string) (*RefreshRecord, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if r, ok := m.refresh[h]; ok {
		c := *r
		return &c, nil
	}
	return nil, nil
}
func (m *memStore) RevokeRefresh(_ context.Context, h string, now time.Time) (bool, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	r := m.refresh[h]
	if r == nil || r.RevokedAt != nil {
		return false, nil
	}
	r.RevokedAt = &now
	return true, nil
}
func (m *memStore) RevokeFamily(_ context.Context, f string, now time.Time) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	for _, r := range m.refresh {
		if r.FamilyID != f {
			continue
		}
		r.FamilyRevoked = true
		if r.RevokedAt == nil {
			t := now
			r.RevokedAt = &t
		}
	}
	return nil
}

type fixture struct {
	svc   *Service
	store *memStore
	clock *time.Time
}

func setup(t *testing.T) fixture {
	t.Helper()
	st := newMemStore()
	h, _ := bcrypt.GenerateFromPassword([]byte("secret123"), bcrypt.MinCost)
	php := "$2y$" + string(h)[4:]
	st.users["admin"] = &UserRecord{Username: "admin", PasswordHash: string(h), Role: "admin", Active: true, PortalAccess: true}
	st.users["legacy"] = &UserRecord{Username: "legacy", PasswordHash: php, Role: "", Active: true, PortalAccess: true}
	st.users["noportal"] = &UserRecord{Username: "noportal", PasswordHash: string(h), Role: "user", Active: true}
	st.users["viewer"] = &UserRecord{Username: "viewer", PasswordHash: string(h), Role: "viewer", Active: true, PortalAccess: true, ViewerAccounts: []string{"a", "b"}}

	now := time.Date(2026, 10, 7, 9, 0, 0, 0, time.UTC)
	tok := NewTokens("0123456789abcdef0123456789abcdef", "test", 15*time.Minute)
	svc := NewService(st, tok, Options{MaxFailedLogins: 5, LockDuration: 15 * time.Minute, RefreshTTL: 8 * time.Hour})
	svc.now = func() time.Time { return now }
	return fixture{svc: svc, store: st, clock: &now}
}

func status(t *testing.T, err error) int {
	t.Helper()
	e, ok := domain.AsError(err)
	require.True(t, ok, "expected domain error, got %v", err)
	return e.Status
}

func TestLoginSuccessAndToken(t *testing.T) {
	f := setup(t)
	s, err := f.svc.Login(context.Background(), "admin", "secret123", ClientInfo{})
	require.NoError(t, err)
	require.Equal(t, domain.RoleAdmin, s.Principal.Role)
	require.NotEmpty(t, s.AccessToken)
	require.Equal(t, f.clock.Add(15*time.Minute), s.AccessExpiresAt)
	require.Equal(t, f.clock.Add(8*time.Hour), s.RefreshExpiresAt)

	rec, _ := f.store.FindRefresh(context.Background(), HashToken(s.RefreshToken))
	require.NotNil(t, rec, "chỉ lưu bản băm refresh token")
}

func TestTokensRoundTrip(t *testing.T) {
	tok := NewTokens("0123456789abcdef0123456789abcdef", "test", 15*time.Minute)
	s, _, err := tok.Sign(domain.Principal{Username: "v", Role: domain.RoleViewer, ViewerAccounts: []string{"a"}}, time.Now())
	require.NoError(t, err)
	p, err := tok.Verify(s)
	require.NoError(t, err)
	require.Equal(t, "v", p.Username)
	require.Equal(t, []string{"a"}, p.ViewerAccounts)

	other := NewTokens("ffffffffffffffffffffffffffffffff", "test", 15*time.Minute)
	_, err = other.Verify(s)
	require.ErrorIs(t, err, ErrInvalidToken)

	expired, _, _ := tok.Sign(domain.Principal{Username: "v"}, time.Now().Add(-time.Hour))
	_, err = tok.Verify(expired)
	require.ErrorIs(t, err, ErrInvalidToken)
}

func TestLoginPHPBcryptAndDefaultRole(t *testing.T) {
	f := setup(t)
	s, err := f.svc.Login(context.Background(), "legacy", "secret123", ClientInfo{})
	require.NoError(t, err)
	require.Equal(t, domain.RoleUser, s.Principal.Role)
}

func TestLoginWrongPasswordAndUnknownUser(t *testing.T) {
	f := setup(t)
	_, err := f.svc.Login(context.Background(), "admin", "bad", ClientInfo{})
	require.Equal(t, http.StatusUnauthorized, status(t, err))
	_, err = f.svc.Login(context.Background(), "ghost", "bad", ClientInfo{})
	require.Equal(t, http.StatusUnauthorized, status(t, err))
}

func TestLoginNoPortalAccess(t *testing.T) {
	f := setup(t)
	_, err := f.svc.Login(context.Background(), "noportal", "secret123", ClientInfo{})
	require.Equal(t, http.StatusForbidden, status(t, err))
	_, err = f.svc.Login(context.Background(), "noportal", "bad", ClientInfo{})
	require.Equal(t, http.StatusUnauthorized, status(t, err), "sai mật khẩu không được lộ trạng thái portal_access")
}

func TestLockAfterFiveFailures(t *testing.T) {
	f := setup(t)
	ctx := context.Background()
	for i := 0; i < 4; i++ {
		_, err := f.svc.Login(ctx, "admin", "bad", ClientInfo{})
		require.Equal(t, http.StatusUnauthorized, status(t, err))
	}
	_, err := f.svc.Login(ctx, "admin", "bad", ClientInfo{})
	require.Equal(t, http.StatusLocked, status(t, err))

	_, err = f.svc.Login(ctx, "admin", "secret123", ClientInfo{})
	require.Equal(t, http.StatusLocked, status(t, err), "đúng mật khẩu vẫn bị khoá")

	*f.clock = f.clock.Add(16 * time.Minute)
	_, err = f.svc.Login(ctx, "admin", "secret123", ClientInfo{})
	require.NoError(t, err)
}

func TestRefreshRotationAndReuseDetection(t *testing.T) {
	f := setup(t)
	ctx := context.Background()
	s1, err := f.svc.Login(ctx, "viewer", "secret123", ClientInfo{})
	require.NoError(t, err)

	s2, err := f.svc.Refresh(ctx, s1.RefreshToken, ClientInfo{})
	require.NoError(t, err)
	require.NotEqual(t, s1.RefreshToken, s2.RefreshToken)
	require.Equal(t, []string{"a", "b"}, s2.Principal.ViewerAccounts)

	// Trong khoảng grace: request song song dùng lại token cũ vẫn được.
	_, err = f.svc.Refresh(ctx, s1.RefreshToken, ClientInfo{})
	require.NoError(t, err)

	// Quá grace: dùng lại token cũ → thu hồi cả chuỗi, token mới cũng mất hiệu lực.
	*f.clock = f.clock.Add(time.Minute)
	_, err = f.svc.Refresh(ctx, s1.RefreshToken, ClientInfo{})
	require.Equal(t, http.StatusUnauthorized, status(t, err))
	*f.clock = f.clock.Add(time.Minute)
	_, err = f.svc.Refresh(ctx, s2.RefreshToken, ClientInfo{})
	require.Equal(t, http.StatusUnauthorized, status(t, err))
}

func TestRefreshExpiredAndUserDisabled(t *testing.T) {
	f := setup(t)
	ctx := context.Background()
	s1, _ := f.svc.Login(ctx, "admin", "secret123", ClientInfo{})

	f.store.users["admin"].PortalAccess = false
	_, err := f.svc.Refresh(ctx, s1.RefreshToken, ClientInfo{})
	require.Equal(t, http.StatusUnauthorized, status(t, err))

	f.store.users["admin"].PortalAccess = true
	s2, _ := f.svc.Login(ctx, "admin", "secret123", ClientInfo{})
	*f.clock = f.clock.Add(9 * time.Hour)
	_, err = f.svc.Refresh(ctx, s2.RefreshToken, ClientInfo{})
	require.Equal(t, http.StatusUnauthorized, status(t, err))
}

func TestLogoutRevokesFamily(t *testing.T) {
	f := setup(t)
	ctx := context.Background()
	s1, _ := f.svc.Login(ctx, "admin", "secret123", ClientInfo{})
	s2, _ := f.svc.Refresh(ctx, s1.RefreshToken, ClientInfo{})
	require.NoError(t, f.svc.Logout(ctx, s2.RefreshToken))
	_, err := f.svc.Refresh(ctx, s2.RefreshToken, ClientInfo{})
	require.Error(t, err)
	require.NoError(t, f.svc.Logout(ctx, "rt_unknown_token_value_xxxx"))
}

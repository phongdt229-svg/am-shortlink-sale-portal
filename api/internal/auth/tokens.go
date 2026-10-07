package auth

import (
	"errors"
	"time"

	"github.com/golang-jwt/jwt/v5"

	"am-shortlink-portal/api/internal/domain"
)

// Tokens ký / kiểm JWT truy cập (HS256). JWT chỉ nằm ở BFF (server), trình duyệt không thấy.
type Tokens struct {
	key    []byte
	issuer string
	ttl    time.Duration
}

func NewTokens(key, issuer string, ttl time.Duration) *Tokens {
	return &Tokens{key: []byte(key), issuer: issuer, ttl: ttl}
}

type claims struct {
	Role     string   `json:"role"`
	Email    string   `json:"email,omitempty"`
	Accounts []string `json:"acc,omitempty"` // viewer_accounts (chỉ viewer)
	jwt.RegisteredClaims
}

const audience = "portal-api"

func (t *Tokens) Sign(p domain.Principal, now time.Time) (string, time.Time, error) {
	exp := now.Add(t.ttl)
	c := claims{
		Role:     string(p.Role),
		Email:    p.Email,
		Accounts: p.ViewerAccounts,
		RegisteredClaims: jwt.RegisteredClaims{
			Issuer:    t.issuer,
			Subject:   p.Username,
			Audience:  jwt.ClaimStrings{audience},
			IssuedAt:  jwt.NewNumericDate(now),
			NotBefore: jwt.NewNumericDate(now.Add(-30 * time.Second)),
			ExpiresAt: jwt.NewNumericDate(exp),
		},
	}
	s, err := jwt.NewWithClaims(jwt.SigningMethodHS256, c).SignedString(t.key)
	return s, exp, err
}

var ErrInvalidToken = errors.New("invalid token")

func (t *Tokens) Verify(token string) (domain.Principal, error) {
	var c claims
	_, err := jwt.ParseWithClaims(token, &c, func(*jwt.Token) (any, error) { return t.key, nil },
		jwt.WithValidMethods([]string{jwt.SigningMethodHS256.Alg()}),
		jwt.WithIssuer(t.issuer),
		jwt.WithAudience(audience),
		jwt.WithExpirationRequired(),
		jwt.WithLeeway(5*time.Second),
	)
	if err != nil || c.Subject == "" {
		return domain.Principal{}, ErrInvalidToken
	}
	p := domain.Principal{Username: c.Subject, Email: c.Email, Role: domain.ParseRole(c.Role)}
	if p.Role == domain.RoleViewer {
		p.ViewerAccounts = c.Accounts
	}
	return p, nil
}

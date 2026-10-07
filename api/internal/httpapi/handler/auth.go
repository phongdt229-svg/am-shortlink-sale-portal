package handler

import (
	"context"

	"am-shortlink-portal/api/internal/auth"
	"am-shortlink-portal/api/internal/httpapi/gen"
)

func tokenPair(s *auth.Session) gen.TokenPair {
	return gen.TokenPair{
		AccessToken:      s.AccessToken,
		AccessExpiresAt:  s.AccessExpiresAt.UTC(),
		RefreshToken:     s.RefreshToken,
		RefreshExpiresAt: s.RefreshExpiresAt.UTC(),
		User:             toMe(s.Principal),
	}
}

func (h *Handler) Login(ctx context.Context, req gen.LoginRequestObject) (gen.LoginResponseObject, error) {
	s, err := h.Auth.Login(ctx, req.Body.Username, req.Body.Password, clientInfo(ctx))
	if err != nil {
		return nil, err
	}
	return gen.Login200JSONResponse(tokenPair(s)), nil
}

func (h *Handler) RefreshToken(ctx context.Context, req gen.RefreshTokenRequestObject) (gen.RefreshTokenResponseObject, error) {
	s, err := h.Auth.Refresh(ctx, req.Body.RefreshToken, clientInfo(ctx))
	if err != nil {
		return nil, err
	}
	return gen.RefreshToken200JSONResponse(tokenPair(s)), nil
}

func (h *Handler) Logout(ctx context.Context, req gen.LogoutRequestObject) (gen.LogoutResponseObject, error) {
	if err := h.Auth.Logout(ctx, req.Body.RefreshToken); err != nil {
		return nil, err
	}
	return gen.Logout204Response{}, nil
}

func (h *Handler) GetMe(ctx context.Context, _ gen.GetMeRequestObject) (gen.GetMeResponseObject, error) {
	p, err := principal(ctx)
	if err != nil {
		return nil, err
	}
	return gen.GetMe200JSONResponse(toMe(p)), nil
}

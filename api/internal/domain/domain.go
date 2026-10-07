// Package domain chứa kiểu nghiệp vụ thuần, không phụ thuộc MongoDB / HTTP.
package domain

import (
	"context"
	"errors"
	"net/http"
)

type Role string

const (
	RoleAdmin  Role = "admin"
	RoleUser   Role = "user"
	RoleViewer Role = "viewer"
)

// ParseRole: giá trị lạ → user (quyền hẹp nhất có dữ liệu của chính mình).
func ParseRole(s string) Role {
	switch Role(s) {
	case RoleAdmin, RoleViewer:
		return Role(s)
	default:
		return RoleUser
	}
}

// Principal là người dùng đã xác thực của một request.
type Principal struct {
	Username string
	Email    string
	Role     Role
	// ViewerAccounts chỉ có ý nghĩa với viewer.
	ViewerAccounts []string
}

func (p Principal) IsAdmin() bool { return p.Role == RoleAdmin }

type principalKey struct{}

func WithPrincipal(ctx context.Context, p Principal) context.Context {
	return context.WithValue(ctx, principalKey{}, p)
}

func PrincipalFrom(ctx context.Context) (Principal, bool) {
	p, ok := ctx.Value(principalKey{}).(Principal)
	return p, ok
}

// Error là lỗi nghiệp vụ có HTTP status + mã máy đọc được; httpapi chuyển thành problem+json.
type Error struct {
	Status int
	Code   string
	Title  string
	Detail string
	Fields []FieldError
}

type FieldError struct {
	Field   string
	Message string
}

func (e *Error) Error() string {
	if e.Detail != "" {
		return e.Code + ": " + e.Detail
	}
	return e.Code
}

func newErr(status int, code, title, detail string) *Error {
	return &Error{Status: status, Code: code, Title: title, Detail: detail}
}

func BadRequest(code, detail string) *Error {
	return newErr(http.StatusBadRequest, code, "Yêu cầu không hợp lệ", detail)
}
func Unauthorized(code, detail string) *Error {
	return newErr(http.StatusUnauthorized, code, "Chưa xác thực", detail)
}
func Forbidden(code, detail string) *Error {
	return newErr(http.StatusForbidden, code, "Không có quyền", detail)
}
func NotFound(code, detail string) *Error {
	return newErr(http.StatusNotFound, code, "Không tìm thấy", detail)
}
func Conflict(code, detail string) *Error {
	return newErr(http.StatusConflict, code, "Xung đột", detail)
}
func Locked(code, detail string) *Error {
	return newErr(http.StatusLocked, code, "Tạm khoá", detail)
}
func TooManyRequests(code, detail string) *Error {
	return newErr(http.StatusTooManyRequests, code, "Quá nhiều yêu cầu", detail)
}
func Unprocessable(code, detail string) *Error {
	return newErr(http.StatusUnprocessableEntity, code, "Không xử lý được", detail)
}

// AsError lấy *Error trong chuỗi lỗi.
func AsError(err error) (*Error, bool) {
	var e *Error
	ok := errors.As(err, &e)
	return e, ok
}

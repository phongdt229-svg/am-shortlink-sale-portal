// Package scope tính phạm vi dữ liệu theo vai trò và ép điều kiện vào MỌI truy vấn MongoDB.
//
//	admin  = toàn bộ
//	user   = owner = chính mình
//	viewer = owner ∈ viewer_accounts
//
// Tham số owner / account ngoài phạm vi → 403 (không âm thầm bỏ qua).
package scope

import (
	"slices"

	"go.mongodb.org/mongo-driver/v2/bson"

	"am-shortlink-portal/api/internal/domain"
)

type Scope struct {
	all      bool
	accounts []string
}

// For dựng phạm vi từ người dùng hiện tại.
func For(p domain.Principal) Scope {
	switch p.Role {
	case domain.RoleAdmin:
		return Scope{all: true}
	case domain.RoleViewer:
		acc := slices.Clone(p.ViewerAccounts)
		slices.Sort(acc)
		return Scope{accounts: slices.Compact(acc)}
	default:
		return Scope{accounts: []string{p.Username}}
	}
}

// All: không giới hạn tài khoản.
func (s Scope) All() bool { return s.all }

// Accounts: danh sách tài khoản được xem (rỗng khi All).
func (s Scope) Accounts() []string { return slices.Clone(s.accounts) }

// Allows: tài khoản có nằm trong phạm vi không.
func (s Scope) Allows(owner string) bool {
	if s.all {
		return true
	}
	_, ok := slices.BinarySearch(s.accounts, owner) // accounts luôn đã sort (For)
	return ok
}

// Resolve giao danh sách tài khoản yêu cầu với phạm vi.
//
//	unrestricted = true  → không cần điều kiện owner (admin, không lọc tài khoản)
//	owners               → danh sách owner phải ép vào truy vấn (có thể rỗng = không thấy gì)
func (s Scope) Resolve(requested []string) (owners []string, unrestricted bool, err error) {
	requested = dedupe(requested)
	if len(requested) == 0 {
		if s.all {
			return nil, true, nil
		}
		return s.Accounts(), false, nil
	}
	for _, r := range requested {
		if !s.Allows(r) {
			return nil, false, domain.Forbidden("forbidden_scope", "tài khoản "+r+" nằm ngoài phạm vi được xem")
		}
	}
	return requested, false, nil
}

// Filter trả điều kiện bson cho trường owner (vd "owner", "meta.owner", "owner_username").
// Kết quả rỗng nghĩa là không cần điều kiện (admin xem tất cả).
func (s Scope) Filter(field string, requested []string) (bson.D, error) {
	owners, unrestricted, err := s.Resolve(requested)
	if err != nil {
		return nil, err
	}
	if unrestricted {
		return bson.D{}, nil
	}
	if len(owners) == 1 {
		return bson.D{{Key: field, Value: owners[0]}}, nil
	}
	if owners == nil {
		owners = []string{} // nil → BSON null → Mongo lỗi "$in needs an array"
	}
	return bson.D{{Key: field, Value: bson.D{{Key: "$in", Value: owners}}}}, nil
}

// RequireAdmin chặn chức năng chỉ dành cho admin.
func RequireAdmin(p domain.Principal) error {
	if !p.IsAdmin() {
		return domain.Forbidden("admin_only", "chức năng chỉ dành cho admin")
	}
	return nil
}

func dedupe(in []string) []string {
	out := make([]string, 0, len(in))
	for _, v := range in {
		if v != "" && !slices.Contains(out, v) {
			out = append(out, v)
		}
	}
	return out
}

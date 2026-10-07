// Package mask che dữ liệu nhạy cảm trước khi trả cho frontend.
//
//   - SĐT CTV: 0901234052 → 090****052 (mọi vai trò trừ admin).
//   - IP: 113.161.20.5 → 113.161.x.x (đủ với admin).
//   - CTV ref: mã tham chiếu mờ (mã hoá xác định) thay cho ctv_id trên URL, để không lộ SĐT qua đường dẫn.
//   - PII hash: SHA-256 có salt cho tham số pii=hash (chỉ tra cứu chính xác).
package mask

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"net/netip"
	"strings"
)

// NormalizePhone chuẩn hoá SĐT về dạng 0xxxxxxxxx; giá trị không phải SĐT (hash) giữ nguyên (trim).
func NormalizePhone(s string) string {
	s = strings.TrimSpace(s)
	var b strings.Builder
	for _, r := range s {
		if r >= '0' && r <= '9' {
			b.WriteRune(r)
		} else if r != ' ' && r != '.' && r != '-' && r != '+' && r != '(' && r != ')' {
			return s // có ký tự khác số → không phải SĐT
		}
	}
	d := b.String()
	switch {
	case len(d) == 11 && strings.HasPrefix(d, "84"):
		d = "0" + d[2:]
	case len(d) == 9 && d[0] != '0':
		d = "0" + d
	}
	if len(d) == 10 && d[0] == '0' {
		return d
	}
	return s
}

// IsPhone: ctv_id có phải SĐT đã chuẩn hoá.
func IsPhone(s string) bool {
	if len(s) != 10 || s[0] != '0' {
		return false
	}
	for _, r := range s {
		if r < '0' || r > '9' {
			return false
		}
	}
	return true
}

// Phone che SĐT: giữ 3 số đầu, 3 số cuối. Không phải SĐT → giữ nguyên (hash CTV không phải PII).
func Phone(s string) string {
	if !IsPhone(s) {
		return s
	}
	return s[:3] + "****" + s[7:]
}

// IP che IP: IPv4 giữ 2 octet đầu; IPv6 giữ 2 nhóm đầu.
func IP(s string) string {
	if s == "" {
		return ""
	}
	a, err := netip.ParseAddr(strings.TrimSpace(s))
	if err != nil {
		return "x.x.x.x"
	}
	a = a.Unmap()
	if a.Is4() {
		b := a.As4()
		return itoa(b[0]) + "." + itoa(b[1]) + ".x.x"
	}
	parts := strings.Split(a.StringExpanded(), ":")
	return strings.TrimLeft(parts[0], "0") + ":" + strings.TrimLeft(parts[1], "0") + ":x::"
}

func itoa(b byte) string {
	if b == 0 {
		return "0"
	}
	var buf [3]byte
	i := len(buf)
	for b > 0 {
		i--
		buf[i] = '0' + b%10
		b /= 10
	}
	return string(buf[i:])
}

// Masker giữ khoá dẫn xuất từ PII_HASH_SALT.
type Masker struct {
	salt   []byte
	refKey []byte // AES-256
	ivKey  []byte // HMAC cho nonce xác định
}

func NewMasker(salt string) *Masker {
	return &Masker{
		salt:   []byte(salt),
		refKey: derive(salt, "ctv-ref-enc"),
		ivKey:  derive(salt, "ctv-ref-iv"),
	}
}

func derive(secret, label string) []byte {
	h := hmac.New(sha256.New, []byte(secret))
	h.Write([]byte(label))
	return h.Sum(nil)
}

// HashPII: hex(SHA-256(salt || value)) — cùng công thức với Service khi ghi tham số pii=hash.
func (m *Masker) HashPII(value string) string {
	h := sha256.New()
	h.Write(m.salt)
	h.Write([]byte(value))
	return hex.EncodeToString(h.Sum(nil))
}

// CTVRef mã hoá xác định ctv_id → chuỗi base64url an toàn cho URL.
func (m *Masker) CTVRef(ctvID string) string {
	if ctvID == "" {
		return ""
	}
	gcm := m.gcm()
	mac := hmac.New(sha256.New, m.ivKey)
	mac.Write([]byte(ctvID))
	nonce := mac.Sum(nil)[:gcm.NonceSize()]
	out := gcm.Seal(append([]byte{}, nonce...), nonce, []byte(ctvID), nil)
	return base64.RawURLEncoding.EncodeToString(out)
}

var ErrInvalidRef = errors.New("ctv_ref không hợp lệ")

// ParseCTVRef giải mã ctv_ref; sai / bị sửa → ErrInvalidRef.
func (m *Masker) ParseCTVRef(ref string) (string, error) {
	raw, err := base64.RawURLEncoding.DecodeString(ref)
	if err != nil {
		return "", ErrInvalidRef
	}
	gcm := m.gcm()
	if len(raw) < gcm.NonceSize()+gcm.Overhead() {
		return "", ErrInvalidRef
	}
	nonce, ct := raw[:gcm.NonceSize()], raw[gcm.NonceSize():]
	pt, err := gcm.Open(nil, nonce, ct, nil)
	if err != nil {
		return "", ErrInvalidRef
	}
	return string(pt), nil
}

func (m *Masker) gcm() cipher.AEAD {
	block, err := aes.NewCipher(m.refKey)
	if err != nil {
		panic(err) // khoá 32 byte cố định — không thể lỗi
	}
	g, err := cipher.NewGCM(block)
	if err != nil {
		panic(err)
	}
	return g
}

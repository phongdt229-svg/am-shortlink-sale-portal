package report

import (
	"net/url"
	"strings"

	"am-shortlink-portal/api/internal/mask"
)

// piiSnapshot: bảng PII hiện có (không gọi DB — dùng trong vòng lặp map DTO; được nạp ở lần gọi pii(ctx) gần nhất).
func (s *Service) piiSnapshot() map[string]string {
	s.piiMu.Lock()
	defer s.piiMu.Unlock()
	if s.piiKeys == nil {
		return map[string]string{"phonenumber": "hash", "fullname": "drop", "email": "hash", "guestid": "hash"}
	}
	return s.piiKeys
}

// maskURL: che giá trị tham số nhạy cảm trong URL, giữ nguyên phần còn lại (đường dẫn, tham số thường).
func maskURL(raw string, pii map[string]string, admin bool) string {
	u, err := url.Parse(raw)
	if err != nil || u.RawQuery == "" {
		return raw
	}
	parts := strings.Split(u.RawQuery, "&")
	changed := false
	for i, p := range parts {
		k, v, ok := strings.Cut(p, "=")
		if !ok {
			continue
		}
		key, err := url.QueryUnescape(k)
		if err != nil {
			continue
		}
		key = strings.ToLower(key)
		switch {
		case pii[key] != "":
			parts[i] = k + "=***"
			changed = true
		case key == "utm_extra_ctv" && !admin:
			val, err := url.QueryUnescape(v)
			if err != nil {
				parts[i] = k + "=***"
			} else {
				parts[i] = k + "=" + url.QueryEscape(mask.Phone(mask.NormalizePhone(val)))
			}
			changed = true
		}
	}
	if !changed {
		return raw
	}
	u.RawQuery = strings.Join(parts, "&")
	return u.String()
}

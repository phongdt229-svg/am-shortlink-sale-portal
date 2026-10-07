package report

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestMaskURL(t *testing.T) {
	pii := map[string]string{"phonenumber": "hash", "fullname": "drop"}
	raw := "https://fpt.vn/km?utm_source=zalo&utm_extra_ctv=84932429379&phonenumber=0901234567&FullName=Nguyen%20A#x"

	user := maskURL(raw, pii, false)
	require.NotContains(t, user, "932429379")
	require.NotContains(t, user, "0901234567")
	require.NotContains(t, user, "Nguyen")
	require.Contains(t, user, "utm_source=zalo")
	require.Contains(t, user, "utm_extra_ctv=093%2A%2A%2A%2A379")

	adm := maskURL(raw, pii, true)
	require.Contains(t, adm, "utm_extra_ctv=84932429379", "admin thấy CTV đầy đủ")
	require.NotContains(t, adm, "0901234567", "PII không bao giờ trả bản rõ, kể cả admin")

	require.Equal(t, "https://fpt.vn/a?b=1", maskURL("https://fpt.vn/a?b=1", pii, false))
	require.Equal(t, "not a url %%", maskURL("not a url %%", pii, false))
}

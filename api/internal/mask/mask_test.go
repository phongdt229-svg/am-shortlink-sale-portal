package mask

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestNormalizePhone(t *testing.T) {
	cases := map[string]string{
		"0901234052":       "0901234052",
		"+84 901 234 052":  "0901234052",
		"84901234052":      "0901234052",
		"901234052":        "0901234052",
		"090.123.4052":     "0901234052",
		" abc123hash ":     "abc123hash",
		"12345":            "12345",
		"e3b0c44298fc1c14": "e3b0c44298fc1c14",
	}
	for in, want := range cases {
		require.Equal(t, want, NormalizePhone(in), in)
	}
}

func TestPhone(t *testing.T) {
	require.Equal(t, "090****052", Phone("0901234052"))
	require.Equal(t, "a1b2c3", Phone("a1b2c3"), "hash giữ nguyên")
}

func TestIP(t *testing.T) {
	require.Equal(t, "113.161.x.x", IP("113.161.20.5"))
	require.Equal(t, "10.0.x.x", IP("::ffff:10.0.3.4"))
	require.Equal(t, "2001:db8:x::", IP("2001:0db8:85a3::8a2e:370:7334"))
	require.Equal(t, "x.x.x.x", IP("not-an-ip"))
	require.Equal(t, "", IP(""))
}

func TestCTVRefRoundTripAndDeterministic(t *testing.T) {
	m := NewMasker("0123456789abcdef-salt")
	ref := m.CTVRef("0901234052")
	require.Equal(t, ref, m.CTVRef("0901234052"), "xác định để URL ổn định")
	require.NotContains(t, ref, "0901234052")

	got, err := m.ParseCTVRef(ref)
	require.NoError(t, err)
	require.Equal(t, "0901234052", got)

	_, err = m.ParseCTVRef(ref[:len(ref)-2] + "AA")
	require.ErrorIs(t, err, ErrInvalidRef)

	other := NewMasker("another-salt-0123456789")
	_, err = other.ParseCTVRef(ref)
	require.ErrorIs(t, err, ErrInvalidRef)
}

func TestHashPII(t *testing.T) {
	m := NewMasker("salt-salt-salt-salt")
	require.Len(t, m.HashPII("x"), 64)
	require.Equal(t, m.HashPII("x"), m.HashPII("x"))
	require.NotEqual(t, m.HashPII("x"), m.HashPII("y"))
}

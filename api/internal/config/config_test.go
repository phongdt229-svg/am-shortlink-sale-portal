package config

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestLoadFileSecrets_PrefersFile(t *testing.T) {
	env := map[string]string{
		"MONGODB_URI":      "mongodb://from-env",
		"MONGODB_URI_FILE": "/vault/mongo",
	}
	files := map[string]string{"/vault/mongo": "mongodb://from-file\n"}

	err := loadFileSecrets(
		func(k string) string { return env[k] },
		func(p string) ([]byte, error) {
			v, ok := files[p]
			if !ok {
				return nil, errors.New("missing")
			}
			return []byte(v), nil
		},
		func(k, v string) error { env[k] = v; return nil },
	)
	require.NoError(t, err)
	require.Equal(t, "mongodb://from-file", env["MONGODB_URI"])
}

func TestLoad_MissingRequiredReportsNameOnly(t *testing.T) {
	t.Setenv("MONGODB_URI", "")
	t.Setenv("PORTAL_JWT_SIGNING_KEY", "")
	t.Setenv("PII_HASH_SALT", "")
	_, err := Load()
	require.Error(t, err)
	require.Contains(t, err.Error(), "MONGODB_URI")
}

func TestLoad_ValidatesKeyLength(t *testing.T) {
	t.Setenv("MONGODB_URI", "mongodb://localhost")
	t.Setenv("PORTAL_JWT_SIGNING_KEY", "short")
	t.Setenv("PII_HASH_SALT", "0123456789abcdef")
	_, err := Load()
	require.ErrorContains(t, err, "PORTAL_JWT_SIGNING_KEY")
	require.NotContains(t, err.Error(), "short")
}

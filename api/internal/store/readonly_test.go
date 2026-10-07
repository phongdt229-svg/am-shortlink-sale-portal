package store

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

// portal-api CHỈ ĐỌC am_shortlink (dữ liệu nghiệp vụ của Service). Kiểm tĩnh: không lời gọi ghi nào đi qua s.core(...).
// (Lớp bảo vệ thứ 2 là role MongoDB portal_api — deploy/mongo/portal_api_role.js.)
func TestNoWritesToCoreDatabase(t *testing.T) {
	writeOps := regexp.MustCompile(`s\.core\([^)]*\)\.(InsertOne|InsertMany|UpdateOne|UpdateMany|UpdateByID|ReplaceOne|DeleteOne|DeleteMany|FindOneAndUpdate|FindOneAndReplace|FindOneAndDelete|BulkWrite|Drop|Indexes)\(`)
	files, err := filepath.Glob("*.go")
	require.NoError(t, err)
	for _, f := range files {
		if strings.HasSuffix(f, "_test.go") {
			continue
		}
		b, err := os.ReadFile(f)
		require.NoError(t, err)
		for i, line := range strings.Split(string(b), "\n") {
			require.False(t, writeOps.MatchString(line), "%s:%d ghi vào am_shortlink: %s", f, i+1, strings.TrimSpace(line))
		}
	}
}

// Ghi vào am_shortlink_report chỉ được phép trên collection của Portal + param_registry (§3.1).
func TestReportWritesOnlyPortalCollections(t *testing.T) {
	writeOps := regexp.MustCompile(`s\.report\(([A-Za-z]+)\)\.(InsertOne|InsertMany|UpdateOne|UpdateMany|ReplaceOne|DeleteOne|DeleteMany|FindOneAndUpdate|FindOneAndDelete|BulkWrite)\(`)
	allowed := map[string]bool{
		"CollRefreshTokens": true, "CollLoginAttempts": true, "CollSavedReports": true, "CollExports": true,
		"CollAuditLog": true, "CollMigrations": true, "CollParamRegistry": true,
	}
	files, _ := filepath.Glob("*.go")
	for _, f := range files {
		if strings.HasSuffix(f, "_test.go") {
			continue
		}
		b, _ := os.ReadFile(f)
		for i, line := range strings.Split(string(b), "\n") {
			if m := writeOps.FindStringSubmatch(line); m != nil {
				require.True(t, allowed[m[1]], "%s:%d ghi vào %s (không thuộc Portal)", f, i+1, m[1])
			}
		}
	}
}

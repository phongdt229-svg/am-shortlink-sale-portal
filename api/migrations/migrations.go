// Package migrations nhúng các file migration của Portal (collection + index trong am_shortlink_report).
// Thêm file mới: NNNN_mo_ta.json với version tăng dần; không sửa file đã chạy.
package migrations

import (
	"embed"
	"encoding/json"
	"fmt"
	"io/fs"
	"sort"
)

//go:embed *.json
var files embed.FS

type Index struct {
	Collection         string         `json:"collection"`
	Name               string         `json:"name"`
	Keys               [][2]any       `json:"keys"`
	Unique             bool           `json:"unique,omitempty"`
	ExpireAfterSeconds *int32         `json:"expire_after_seconds,omitempty"`
	Partial            map[string]any `json:"partial,omitempty"`
}

type Migration struct {
	Version     int      `json:"version"`
	Description string   `json:"description"`
	Collections []string `json:"collections"`
	Indexes     []Index  `json:"indexes"`
}

// All trả các migration theo version tăng dần.
func All() ([]Migration, error) {
	names, err := fs.Glob(files, "*.json")
	if err != nil {
		return nil, err
	}
	var out []Migration
	seen := map[int]string{}
	for _, n := range names {
		b, err := files.ReadFile(n)
		if err != nil {
			return nil, err
		}
		var m Migration
		if err := json.Unmarshal(b, &m); err != nil {
			return nil, fmt.Errorf("%s: %w", n, err)
		}
		if prev, dup := seen[m.Version]; dup {
			return nil, fmt.Errorf("trùng version %d: %s và %s", m.Version, prev, n)
		}
		seen[m.Version] = n
		out = append(out, m)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Version < out[j].Version })
	return out, nil
}

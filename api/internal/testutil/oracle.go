package testutil

import (
	"slices"
	"time"

	"am-shortlink-portal/api/internal/seed"
)

// Oracle tính chỉ số TRỰC TIẾP từ click / link thô của dataset theo định nghĩa §5.2 —
// độc lập với stats_* và với code portal-api, dùng làm "đáp án" khi đối chiếu.
type Filter struct {
	From, To  time.Time // date-only
	Owners    []string  // rỗng = tất cả
	Campaigns []string
	CTVs      []string
	Prefixes  []string
}

type Metrics struct {
	Total, New, Active, Clicks, Unique, Bot, Susp int64
}

func match(vals []string, v string) bool { return len(vals) == 0 || slices.Contains(vals, v) }

func (f Filter) link(l *seed.Link) bool {
	return match(f.Owners, l.Owner) && match(f.Campaigns, l.Campaign) && match(f.CTVs, l.CTV) && match(f.Prefixes, l.Prefix)
}

func (f Filter) inRange(t time.Time) bool {
	d := seed.DateOf(t)
	return !d.Before(f.From) && !d.After(f.To)
}

func Compute(ds *seed.Dataset, f Filter) Metrics {
	var m Metrics
	links := map[int64]*seed.Link{}
	end := time.Date(f.To.Year(), f.To.Month(), f.To.Day(), 0, 0, 0, 0, seed.VN).AddDate(0, 0, 1)
	for i := range ds.Links {
		l := &ds.Links[i]
		links[l.ID] = l
		if !f.link(l) {
			continue
		}
		if f.inRange(l.CreatedAt) {
			m.New++
		}
		if l.CreatedAt.Before(end) && l.Status != "deleted" {
			m.Total++
		}
	}
	active := map[int64]bool{}
	for i := range ds.Clicks {
		c := &ds.Clicks[i]
		if !f.inRange(c.TS) || !f.link(links[c.LinkID]) {
			continue
		}
		if c.IsBot {
			m.Bot++
			continue
		}
		m.Clicks++
		if !c.IsRepeat {
			m.Unique++
		}
		if c.IsSuspicious {
			m.Susp++
		}
		active[c.LinkID] = true
	}
	m.Active = int64(len(active))
	return m
}

// ClicksBy: clicks (không bot) theo hàm khoá.
func ClicksBy(ds *seed.Dataset, f Filter, key func(c *seed.Click, l *seed.Link) string) map[string]int64 {
	links := map[int64]*seed.Link{}
	for i := range ds.Links {
		links[ds.Links[i].ID] = &ds.Links[i]
	}
	out := map[string]int64{}
	for i := range ds.Clicks {
		c := &ds.Clicks[i]
		l := links[c.LinkID]
		if c.IsBot || !f.inRange(c.TS) || !f.link(l) {
			continue
		}
		out[key(c, l)]++
	}
	return out
}

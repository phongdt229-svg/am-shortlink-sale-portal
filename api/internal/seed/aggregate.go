package seed

import (
	"sort"
	"strings"
	"time"
)

// EscapeKey: khoá map trong Mongo không nên chứa "." (referer_host) → thay bằng "．" (U+FF0E).
// Portal giải mã bằng store.DecodeKey. (Hợp đồng schema với Service.)
func EscapeKey(k string) string {
	if k == "" {
		return "(direct)"
	}
	return strings.ReplaceAll(k, ".", "．")
}

// acc gom chỉ số theo định nghĩa §5.2: clicks / by_* chỉ tính click KHÔNG phải bot.
type acc struct {
	Clicks, Unique, Bot, Susp, NewLinks int64
	active                              map[int64]bool
	ByHour                              [24]int64
	ByDevice, ByReferer, ByCountry      map[string]int64
	ByOS, ByBrowser, BySource, ByPrefix map[string]int64
	LastClick                           time.Time
	FirstClick                          time.Time
}

func newAcc() *acc {
	return &acc{
		active:   map[int64]bool{},
		ByDevice: map[string]int64{}, ByReferer: map[string]int64{}, ByCountry: map[string]int64{},
		ByOS: map[string]int64{}, ByBrowser: map[string]int64{}, BySource: map[string]int64{}, ByPrefix: map[string]int64{},
	}
}

func (a *acc) add(c *Click) {
	if c.IsBot {
		a.Bot++
		return
	}
	a.Clicks++
	if !c.IsRepeat {
		a.Unique++
	}
	if c.IsSuspicious {
		a.Susp++
	}
	a.active[c.LinkID] = true
	a.ByHour[c.Hour]++
	a.ByDevice[c.Device]++
	a.ByReferer[EscapeKey(c.RefererHost)]++
	a.ByCountry[c.Country]++
	a.ByOS[c.OS]++
	a.ByBrowser[c.Browser]++
	a.BySource[c.SourceGroup]++
	a.ByPrefix[c.AccessPrefix]++
	if a.FirstClick.IsZero() || c.TS.Before(a.FirstClick) {
		a.FirstClick = c.TS
	}
	if c.TS.After(a.LastClick) {
		a.LastClick = c.TS
	}
}

// group: một bảng thống kê (khoá → acc), giữ thứ tự chèn để dữ liệu xác định.
type group struct {
	keys []string
	m    map[string]*acc
	attr map[string]map[string]any
}

func newGroup() *group { return &group{m: map[string]*acc{}, attr: map[string]map[string]any{}} }

func (g *group) get(key string, attr map[string]any) *acc {
	a, ok := g.m[key]
	if !ok {
		a = newAcc()
		g.m[key] = a
		g.keys = append(g.keys, key)
		g.attr[key] = attr
	}
	return a
}

type Stats struct {
	LinkDaily, CampaignDaily, CTVDaily, OwnerDaily, SystemDaily, ParamDaily             *group
	LinkMonthly, CampaignMonthly, CTVMonthly, OwnerMonthly, SystemMonthly, ParamMonthly *group
	ParamValues                                                                         map[string]*paramValue
	ParamValueKeys                                                                      []string
	Facets                                                                              map[string]*facet
	FacetKeys                                                                           []string
	LinkClicks                                                                          map[int64]int64
	TotalSnap                                                                           map[string]int64 // owner|date → total_links_snapshot ("" owner = hệ thống)
}

type paramValue struct {
	Owner, Key, Value   string
	FirstSeen, LastSeen time.Time
	links               map[int64]bool
	ClicksTotal         int64
}

type facet struct {
	Owner, Field, Value string
	Clicks              int64
	LastDate            time.Time
}

func monthOf(d time.Time) time.Time { return time.Date(d.Year(), d.Month(), 1, 0, 0, 0, 0, time.UTC) }

func isTracked(k string) bool {
	for _, t := range TrackedKeys {
		if t == k {
			return true
		}
	}
	return false
}

// Aggregate tính toàn bộ dữ liệu thống kê từ dataset.
func Aggregate(ds *Dataset) *Stats {
	s := &Stats{
		LinkDaily: newGroup(), CampaignDaily: newGroup(), CTVDaily: newGroup(), OwnerDaily: newGroup(), SystemDaily: newGroup(), ParamDaily: newGroup(),
		LinkMonthly: newGroup(), CampaignMonthly: newGroup(), CTVMonthly: newGroup(), OwnerMonthly: newGroup(), SystemMonthly: newGroup(), ParamMonthly: newGroup(),
		ParamValues: map[string]*paramValue{}, Facets: map[string]*facet{}, LinkClicks: map[int64]int64{}, TotalSnap: map[string]int64{},
	}
	links := make(map[int64]*Link, len(ds.Links))
	for i := range ds.Links {
		links[ds.Links[i].ID] = &ds.Links[i]
	}

	type target struct {
		g    *group
		key  string
		attr map[string]any
	}
	targets := func(l *Link, day time.Time, monthly bool) []target {
		k := "date"
		if monthly {
			k = "month"
			day = monthOf(day)
		}
		ds := day.Format("2006-01-02")
		pick := func(d, m *group) *group {
			if monthly {
				return m
			}
			return d
		}
		t := []target{
			{pick(s.LinkDaily, s.LinkMonthly), itoa64(l.ID) + "|" + ds, map[string]any{"link_id": l.ID, k: day, "owner": l.Owner, "campaign_code": l.Campaign, "ctv_id": l.CTV, "prefix": l.Prefix}},
			{pick(s.CampaignDaily, s.CampaignMonthly), l.Owner + "|" + l.Campaign + "|" + ds, map[string]any{"owner": l.Owner, "campaign_code": l.Campaign, k: day}},
			{pick(s.CTVDaily, s.CTVMonthly), l.CTV + "|" + l.Owner + "|" + l.Campaign + "|" + ds, map[string]any{"ctv_id": l.CTV, "owner": l.Owner, "campaign_code": l.Campaign, k: day}},
			{pick(s.OwnerDaily, s.OwnerMonthly), l.Owner + "|" + ds, map[string]any{"owner": l.Owner, k: day}},
			{pick(s.SystemDaily, s.SystemMonthly), ds, map[string]any{k: day}},
		}
		for _, p := range l.Params {
			if isTracked(p.Key) {
				t = append(t, target{pick(s.ParamDaily, s.ParamMonthly),
					l.Owner + "|" + l.Campaign + "|" + p.Key + "|" + p.Value + "|" + ds,
					map[string]any{"owner": l.Owner, "campaign_code": l.Campaign, "key": p.Key, "value": p.Value, k: day}})
			}
		}
		return t
	}

	// Ngày đầu tiên có dữ liệu: link sớm nhất.
	first := ds.To
	for i := range ds.Links {
		if d := DateOf(ds.Links[i].CreatedAt); d.Before(first) {
			first = d
		}
	}
	owners := []string{}
	seenOwner := map[string]bool{}
	for i := range ds.Links {
		if !seenOwner[ds.Links[i].Owner] {
			seenOwner[ds.Links[i].Owner] = true
			owners = append(owners, ds.Links[i].Owner)
		}
	}
	sort.Strings(owners)
	// Tạo sẵn doc owner / system cho mọi ngày (timeline liền mạch, có total_links_snapshot).
	for d := first; !d.After(ds.To); d = d.AddDate(0, 0, 1) {
		dsKey := d.Format("2006-01-02")
		s.SystemDaily.get(dsKey, map[string]any{"date": d})
		s.SystemMonthly.get(monthOf(d).Format("2006-01-02"), map[string]any{"month": monthOf(d)})
		for _, o := range owners {
			s.OwnerDaily.get(o+"|"+dsKey, map[string]any{"owner": o, "date": d})
			s.OwnerMonthly.get(o+"|"+monthOf(d).Format("2006-01-02"), map[string]any{"owner": o, "month": monthOf(d)})
		}
	}

	// new_links + link_params facets
	for i := range ds.Links {
		l := &ds.Links[i]
		day := DateOf(l.CreatedAt)
		for _, monthly := range []bool{false, true} {
			for _, t := range targets(l, day, monthly) {
				if t.g == s.LinkDaily || t.g == s.LinkMonthly {
					continue // stats_link_* không có new_links
				}
				t.g.get(t.key, t.attr).NewLinks++
			}
		}
		for _, p := range l.Params {
			k := l.Owner + "|" + p.Key + "|" + p.Value
			pv, ok := s.ParamValues[k]
			if !ok {
				pv = &paramValue{Owner: l.Owner, Key: p.Key, Value: p.Value, FirstSeen: l.CreatedAt, links: map[int64]bool{}}
				s.ParamValues[k] = pv
				s.ParamValueKeys = append(s.ParamValueKeys, k)
			}
			pv.links[l.ID] = true
			if l.CreatedAt.Before(pv.FirstSeen) {
				pv.FirstSeen = l.CreatedAt
			}
			if l.CreatedAt.After(pv.LastSeen) {
				pv.LastSeen = l.CreatedAt
			}
		}
		addFacet(s, l.Owner, "dest_host", l.DestHost, 0, day)
	}

	// clicks
	for i := range ds.Clicks {
		c := &ds.Clicks[i]
		l := links[c.LinkID]
		day := DateOf(c.TS)
		for _, monthly := range []bool{false, true} {
			for _, t := range targets(l, day, monthly) {
				t.g.get(t.key, t.attr).add(c)
			}
		}
		if c.IsBot {
			continue
		}
		s.LinkClicks[c.LinkID]++
		for _, p := range l.Params {
			s.ParamValues[l.Owner+"|"+p.Key+"|"+p.Value].ClicksTotal++
		}
		for f, v := range map[string]string{
			"device": c.Device, "os": c.OS, "browser": c.Browser, "source_group": c.SourceGroup,
			"referer_host": c.RefererHost, "country": c.Country, "province": c.Province,
			"access_prefix": c.AccessPrefix, "link_api_version": c.APIVersion, "dest_host": c.DestHost,
		} {
			if v != "" {
				addFacet(s, l.Owner, f, v, 1, day)
			}
		}
	}

	// total_links_snapshot: link chưa xoá (tại ngày đó) tạo đến hết ngày.
	for d := first; !d.After(ds.To); d = d.AddDate(0, 0, 1) {
		end := time.Date(d.Year(), d.Month(), d.Day(), 0, 0, 0, 0, VN).Add(24 * time.Hour)
		for i := range ds.Links {
			l := &ds.Links[i]
			if !l.CreatedAt.Before(end) {
				break
			}
			if l.Status == "deleted" && l.StatusAt.Before(end) {
				continue
			}
			s.TotalSnap[l.Owner+"|"+d.Format("2006-01-02")]++
			s.TotalSnap["|"+d.Format("2006-01-02")]++
		}
	}
	sort.Strings(s.FacetKeys)
	return s
}

func addFacet(s *Stats, owner, field, value string, n int64, day time.Time) {
	k := owner + "|" + field + "|" + value
	f, ok := s.Facets[k]
	if !ok {
		f = &facet{Owner: owner, Field: field, Value: value}
		s.Facets[k] = f
		s.FacetKeys = append(s.FacetKeys, k)
	}
	f.Clicks += n
	if day.After(f.LastDate) {
		f.LastDate = day
	}
}

func itoa64(n int64) string {
	if n == 0 {
		return "0"
	}
	neg := n < 0
	if neg {
		n = -n
	}
	var b [20]byte
	i := len(b)
	for n > 0 {
		i--
		b[i] = byte('0' + n%10)
		n /= 10
	}
	if neg {
		i--
		b[i] = '-'
	}
	return string(b[i:])
}

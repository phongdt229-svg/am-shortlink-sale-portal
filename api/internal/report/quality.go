package report

import (
	"context"
	"sort"
	"time"

	openapi_types "github.com/oapi-codegen/runtime/types"

	"am-shortlink-portal/api/internal/domain"
	"am-shortlink-portal/api/internal/httpapi/gen"
	"am-shortlink-portal/api/internal/scope"
	"am-shortlink-portal/api/internal/store"
)

// minClicksForRate: chỉ xếp hạng tỉ lệ lặp khi đủ mẫu.
const minClicksForRate = 20

// TrafficQuality: R7 (admin).
func (s *Service) TrafficQuality(ctx context.Context, q Query) (gen.TrafficQuality, error) {
	if err := scope.RequireAdmin(q.P); err != nil {
		return gen.TrafficQuality{}, err
	}
	cq, err := s.ClickQuery(ctx, q, nil)
	if err != nil {
		return gen.TrafficQuality{}, err
	}
	return cached(s, ctx, "traffic_quality", q, nil, func(ctx context.Context) (gen.TrafficQuality, error) {
		var (
			kpi               gen.Metrics
			ips               []store.IPStat
			bySource          []store.SourceBot
			flagged           []store.ClickDoc
			uas               []string
			ctvRows, campRows map[string]*row
		)
		g, gctx := group(ctx)
		g.Go(func() (err error) { kpi, err = s.metrics(gctx, q.M); return })
		g.Go(func() (err error) { ips, err = s.st.TopIPs(gctx, cq, 50); return })
		g.Go(func() (err error) { bySource, err = s.st.BotBySource(gctx, cq); return })
		g.Go(func() (err error) { flagged, uas, err = s.st.FlaggedClicks(gctx, cq, 100); return })
		g.Go(func() (err error) { ctvRows, err = s.dimRows(gctx, q, store.DimCTV); return })
		g.Go(func() (err error) { campRows, err = s.dimRows(gctx, q, store.DimCampaign); return })
		if err := g.Wait(); err != nil {
			return gen.TrafficQuality{}, err
		}

		out := gen.TrafficQuality{Period: period(q.M)}
		out.Totals.Clicks, out.Totals.BotClicks, out.Totals.SuspiciousClicks = kpi.Clicks, kpi.BotClicks, kpi.SuspiciousClicks
		out.Totals.RepeatClicks = kpi.Clicks - kpi.UniqueClicks

		out.TopIps = make([]gen.IPStat, len(ips))
		for i, ip := range ips {
			last := ip.LastSeen
			country := ip.Country
			out.TopIps[i] = gen.IPStat{Ip: ip.IP, Clicks: ip.Clicks, BotClicks: ip.Bot, SuspiciousClicks: ip.Susp, Links: ip.Links, Accounts: ip.Accounts, Country: &country, LastSeen: &last}
		}

		names, err := s.st.CampaignNames(ctx, keysOf(campRows))
		if err != nil {
			return gen.TrafficQuality{}, err
		}
		out.RepeatByCtv = repeatRows(ctvRows, func(k string) (string, string) { return s.ctvRef(k), q.CTVDisplay(k) })
		out.RepeatByCampaign = repeatRows(campRows, func(k string) (string, string) { return k, names[k] })

		out.BotBySource = make([]gen.SourceBotRow, len(bySource))
		for i, b := range bySource {
			src := b.Source
			if src == "" {
				src = "(không rõ)"
			}
			out.BotBySource[i] = gen.SourceBotRow{SourceGroup: src, Total: b.Total, BotClicks: b.Bot, BotRate: ratio(b.Bot, b.Total)}
		}

		ids := map[int64]bool{}
		for _, c := range flagged {
			ids[c.Meta.LinkID] = true
		}
		list := make([]int64, 0, len(ids))
		for id := range ids {
			list = append(list, id)
		}
		links, err := s.st.LinksByIDs(ctx, q.M, list)
		if err != nil {
			return gen.TrafficQuality{}, err
		}
		rows := s.clickRows(q, flagged, links)
		out.Flagged = make([]gen.FlaggedClick, len(rows))
		for i, r := range rows {
			var reasons []string
			if flagged[i].IsBot {
				reasons = append(reasons, "User-agent là bot: "+truncate(uas[i], 80))
			}
			if flagged[i].IsSuspicious {
				reasons = append(reasons, "IP vượt ngưỡng > 20 click / link / giờ")
			}
			out.Flagged[i] = flaggedFrom(r, reasons)
		}
		return out, nil
	})
}

func flaggedFrom(r gen.ClickRow, reasons []string) gen.FlaggedClick {
	return gen.FlaggedClick{
		AccessPrefix: r.AccessPrefix, Browser: r.Browser, CampaignCode: r.CampaignCode, Code: r.Code, Country: r.Country,
		CtvDisplay: r.CtvDisplay, CtvRef: r.CtvRef, Device: r.Device, Ip: r.Ip, IsBot: r.IsBot, IsRepeat: r.IsRepeat,
		IsSuspicious: r.IsSuspicious, LongUrl: r.LongUrl, Os: r.Os, Owner: r.Owner, Prefix: r.Prefix, Province: r.Province,
		RefererHost: r.RefererHost, SourceGroup: r.SourceGroup, Ts: r.Ts, Reasons: reasons,
	}
}

func keysOf(m map[string]*row) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	return out
}

// repeatRows: top 20 theo tỉ lệ click lặp (đủ mẫu), bỏ dòng chưa định danh / không chiến dịch.
func repeatRows(m map[string]*row, label func(string) (string, string)) []gen.RepeatRow {
	out := []gen.RepeatRow{}
	for k, r := range m {
		if k == "" || r.sums.Clicks < minClicksForRate {
			continue
		}
		key, lbl := label(k)
		if lbl == "" {
			lbl = k
		}
		out = append(out, gen.RepeatRow{Key: key, Label: lbl, Clicks: r.sums.Clicks, UniqueClicks: r.sums.Unique,
			RepeatRate: 1 - ratio(r.sums.Unique, r.sums.Clicks), SuspiciousClicks: r.sums.Susp})
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].RepeatRate != out[j].RepeatRate {
			return out[i].RepeatRate > out[j].RepeatRate
		}
		return out[i].Clicks > out[j].Clicks
	})
	if len(out) > 20 {
		out = out[:20]
	}
	return out
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "…"
}

// ---------- quản lý tham số (§4.1 c3, admin) ----------

func (s *Service) Registry(ctx context.Context, p domain.Principal) (gen.ParamRegistryList, error) {
	if err := scope.RequireAdmin(p); err != nil {
		return gen.ParamRegistryList{}, err
	}
	var (
		reg  []store.RegistryEntry
		seen []store.ParamSeen
	)
	g, gctx := group(ctx)
	g.Go(func() (err error) { reg, err = s.st.Registry(gctx); return })
	g.Go(func() (err error) { seen, err = s.st.ParamsSeen(gctx); return })
	if err := g.Wait(); err != nil {
		return gen.ParamRegistryList{}, err
	}
	byKey := map[string]*gen.ParamRegistryItem{}
	var order []string
	for _, e := range reg {
		it := registryItem(e)
		byKey[e.Key] = &it
		order = append(order, e.Key)
	}
	for _, sp := range seen {
		it, ok := byKey[sp.Key]
		if !ok {
			n := gen.ParamRegistryItem{Key: sp.Key, Label: sp.Key, Pii: "none", Status: "active", InRegistry: ptr(false)}
			it = &n
			byKey[sp.Key] = it
			order = append(order, sp.Key)
		}
		it.Links, it.Values = sp.Links, sp.Values
		samples := sp.Samples
		if it.Pii != "none" {
			samples = []string{"*** (PII)"}
		}
		it.Samples = &samples
	}
	out := gen.ParamRegistryList{Items: make([]gen.ParamRegistryItem, 0, len(order))}
	for _, k := range order {
		out.Items = append(out.Items, *byKey[k])
	}
	sort.SliceStable(out.Items, func(i, j int) bool {
		if out.Items[i].Tracked != out.Items[j].Tracked {
			return out.Items[i].Tracked
		}
		return out.Items[i].Links > out.Items[j].Links
	})
	return out, nil
}

func registryItem(e store.RegistryEntry) gen.ParamRegistryItem {
	pii := e.PII
	if pii == "" {
		pii = "none"
	}
	status := e.Status
	if status == "" {
		status = "active"
	}
	it := gen.ParamRegistryItem{
		Key: e.Key, Label: e.Label, Tracked: e.Tracked, Pii: gen.ParamRegistryItemPii(pii),
		Status: gen.ParamRegistryItemStatus(status), InRegistry: ptr(true),
	}
	if it.Label == "" {
		it.Label = e.Key
	}
	if e.MaxValues > 0 {
		mv := e.MaxValues
		it.MaxValues = &mv
	}
	if e.UpdatedBy != "" {
		it.UpdatedBy = &e.UpdatedBy
		it.UpdatedAt = &e.UpdatedAt
	}
	if b := e.Backfill; b != nil {
		dto := &struct {
			Error    *string             `json:"error,omitempty"`
			From     *openapi_types.Date `json:"from,omitempty"`
			Progress *float64            `json:"progress,omitempty"`
			Status   *string             `json:"status,omitempty"`
		}{Status: ptr(b.Status), Progress: ptr(b.Progress)}
		if !b.From.IsZero() {
			dto.From = &openapi_types.Date{Time: dateOnly(b.From)}
		}
		if b.Error != "" {
			dto.Error = ptr(b.Error)
		}
		it.Backfill = dto
	}
	return it
}

// UpdateRegistry: bật / tắt theo dõi, nhãn, PII, ngưỡng. Bật mới → backfill pending (Service chạy).
func (s *Service) UpdateRegistry(ctx context.Context, p domain.Principal, key string, in gen.ParamRegistryUpdate) (gen.ParamRegistryItem, error) {
	if err := scope.RequireAdmin(p); err != nil {
		return gen.ParamRegistryItem{}, err
	}
	old, err := s.st.RegistryEntry(ctx, key)
	if err != nil {
		return gen.ParamRegistryItem{}, err
	}
	e := store.RegistryEntry{Key: key, Label: key, PII: "none", MaxValues: 1000, Status: "active"}
	if old != nil {
		e = *old
	}
	if in.Label != nil {
		e.Label = *in.Label
	}
	if in.Pii != nil {
		e.PII = string(*in.Pii)
	}
	if in.MaxValues != nil {
		e.MaxValues = *in.MaxValues
	}
	if in.Tracked != nil {
		e.Tracked = *in.Tracked
	}
	if e.Tracked && e.PII != "none" && e.PII != "" {
		return gen.ParamRegistryItem{}, domain.BadRequest("pii_not_trackable", "tham số PII (hash / drop) không bật theo dõi thống kê theo giá trị")
	}
	if e.Status == "disabled" && e.Tracked {
		e.Status = "active"
	}
	e.UpdatedBy = p.Username
	startBackfill := e.Tracked && (old == nil || !old.Tracked)
	saved, err := s.st.UpsertRegistry(ctx, e, startBackfill, time.Now().UTC())
	if err != nil {
		return gen.ParamRegistryItem{}, err
	}
	s.piiMu.Lock()
	s.piiLoaded = time.Time{} // nạp lại bảng PII ở request sau
	s.piiMu.Unlock()
	if s.audit != nil {
		s.audit.Record(ctx, p, "param_registry.update", key, map[string]any{
			"tracked": e.Tracked, "pii": e.PII, "label": e.Label, "max_values": e.MaxValues, "backfill": startBackfill,
		})
	}
	return registryItem(*saved), nil
}

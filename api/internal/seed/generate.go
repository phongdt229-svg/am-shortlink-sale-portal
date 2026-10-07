// Package seed sinh dữ liệu giả lập ĐÚNG hợp đồng schema Service (docs/PLAN_SERVICE_API.md §4, §5.4, §5.4b)
// cho môi trường dev và test tích hợp.
//
// Dữ liệu xác định (cùng Seed → cùng dữ liệu). Mọi stats_* được TỔNG HỢP TỪ CHÍNH click sinh ra,
// theo định nghĩa chỉ số §5.2 — nên đối chiếu được: tổng nhóm Explorer (đọc clicks) = thẻ tổng (đọc stats_*).
//
// Quy ước ngày: trường `date` = 00:00:00Z của NGÀY LỊCH Asia/Ho_Chi_Minh (date-only); `month` = ngày 1 của tháng.
package seed

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"math"
	"math/rand/v2"
	"net/url"
	"sort"
	"strings"
	"time"

	"am-shortlink-portal/api/internal/mask"
)

var VN = mustLoc("Asia/Ho_Chi_Minh")

func mustLoc(name string) *time.Location {
	l, err := time.LoadLocation(name)
	if err != nil {
		return time.FixedZone("ICT", 7*3600) // Windows thiếu tzdata
	}
	return l
}

type Options struct {
	Seed         uint64
	Days         int       // số ngày click (tính cả hôm nay)
	Links        int       // tổng số link
	ClicksPerDay int       // trung bình click / ngày (chưa tính burst)
	Now          time.Time // mốc "bây giờ"
	PIIHashSalt  string
}

func (o *Options) defaults() {
	if o.Seed == 0 {
		o.Seed = 20261007
	}
	if o.Days <= 0 {
		o.Days = 120
	}
	if o.Links <= 0 {
		o.Links = 3000
	}
	if o.ClicksPerDay <= 0 {
		o.ClicksPerDay = 1500
	}
	if o.Now.IsZero() {
		o.Now = time.Now()
	}
}

// ---------- mô hình ----------

type User struct {
	ID             int64
	Username       string
	Email          string
	Password       string // bản rõ — chỉ để in hướng dẫn dev
	Role           string
	Active         bool
	PortalAccess   bool
	ViewerAccounts []string
	Prefix         string
}

type Campaign struct {
	ID        int64
	Code      string
	Name      string
	CreatedBy string
	CreatedAt time.Time
}

type CTV struct {
	ID    string // ctv_id chuẩn hoá
	Name  string
	Owner string
}

type Param struct {
	Key, Value, Raw string
}

type Link struct {
	ID         int64
	Code       string
	LongURL    string
	DestHost   string
	Owner      string
	Campaign   string
	CTVRaw     string
	CTV        string
	Status     string // active | disabled | deleted
	StatusAt   time.Time
	IsCustom   bool
	APIVersion string // v1 | v2 | v3 | portal
	Prefix     string
	CreatedAt  time.Time
	Params     []Param
	weight     float64
}

type Click struct {
	TS           time.Time
	LinkID       int64
	Owner        string
	Campaign     string
	CTV          string
	IP           string
	UA           string
	Country      string
	Province     string
	Device       string
	OS           string
	Browser      string
	InApp        string
	RefererHost  string
	SourceGroup  string
	AccessPrefix string
	LinkPrefix   string
	APIVersion   string
	IsCustom     bool
	LinkCreated  time.Time
	DestHost     string
	UTM          map[string]string
	Hour         int
	Weekday      int // 0 = Chủ nhật (giống Go time.Weekday)
	IsBot        bool
	IsSuspicious bool
	IsRepeat     bool
	EventID      string
}

type Dataset struct {
	Users     []User
	Campaigns []Campaign
	CTVs      []CTV
	Links     []Link
	Clicks    []Click
	Registry  []RegistryEntry
	From, To  time.Time // khoảng ngày click (date-only, gồm cả To)
}

type RegistryEntry struct {
	Key       string
	Label     string
	Tracked   bool
	PII       string // none | hash | drop
	Normalize []string
	MaxValues int
	Status    string
}

// TrackedKeys: tham số theo dõi mặc định (§5.4b).
var TrackedKeys = []string{"utm_source", "utm_medium", "utm_campaign", "utm_content", "utm_term", "utm_extra_ctv"}

func registry() []RegistryEntry {
	r := []RegistryEntry{
		{Key: "utm_source", Label: "Nguồn (utm_source)", Tracked: true, PII: "none", Normalize: []string{"lower", "trim"}, MaxValues: 500, Status: "active"},
		{Key: "utm_medium", Label: "Kênh (utm_medium)", Tracked: true, PII: "none", Normalize: []string{"lower", "trim"}, MaxValues: 500, Status: "active"},
		{Key: "utm_campaign", Label: "Chiến dịch (utm_campaign)", Tracked: true, PII: "none", Normalize: []string{"lower", "trim"}, MaxValues: 2000, Status: "active"},
		{Key: "utm_content", Label: "Nội dung (utm_content)", Tracked: true, PII: "none", Normalize: []string{"trim"}, MaxValues: 2000, Status: "active"},
		{Key: "utm_term", Label: "Từ khoá (utm_term)", Tracked: true, PII: "none", Normalize: []string{"trim"}, MaxValues: 2000, Status: "active"},
		{Key: "utm_extra_ctv", Label: "CTV (utm_extra_ctv)", Tracked: true, PII: "none", Normalize: []string{"phone"}, MaxValues: 100000, Status: "active"},
		{Key: "ref", Label: "Mã giới thiệu", Tracked: false, PII: "none", Normalize: []string{"trim"}, MaxValues: 1000, Status: "active"},
		{Key: "promo", Label: "Mã khuyến mãi", Tracked: false, PII: "none", Normalize: []string{"lower"}, MaxValues: 1000, Status: "active"},
		{Key: "phonenumber", Label: "SĐT khách", Tracked: false, PII: "hash", Normalize: []string{"phone"}, MaxValues: 0, Status: "active"},
		{Key: "gclid", Label: "gclid", Tracked: false, PII: "none", MaxValues: 1000, Status: "high_cardinality"},
	}
	return r
}

// ---------- sinh dữ liệu ----------

func Generate(o Options) *Dataset {
	o.defaults()
	r := rand.New(rand.NewPCG(o.Seed, o.Seed^0x9e3779b97f4a7c15))
	m := mask.NewMasker(o.PIIHashSalt)

	nowVN := o.Now.In(VN)
	today := DateOf(nowVN)
	from := today.AddDate(0, 0, -(o.Days - 1))
	ds := &Dataset{From: from, To: today, Registry: registry()}

	ds.Users = []User{
		{ID: 1, Username: "admin", Email: "admin@example.vn", Password: "admin123", Role: "admin", Active: true, PortalAccess: true, Prefix: "sale"},
		{ID: 2, Username: "viewer", Email: "viewer@example.vn", Password: "viewer123", Role: "viewer", Active: true, PortalAccess: true, ViewerAccounts: []string{"partner_a", "partner_b"}, Prefix: "sale"},
		{ID: 3, Username: "partner_a", Email: "a@partner.vn", Password: "partner123", Role: "user", Active: true, PortalAccess: true, Prefix: "sale"},
		{ID: 4, Username: "partner_b", Email: "b@partner.vn", Password: "partner123", Role: "user", Active: true, PortalAccess: true, Prefix: "lm"},
		{ID: 5, Username: "partner_c", Email: "c@partner.vn", Password: "partner123", Role: "user", Active: true, PortalAccess: true, Prefix: "sale"},
		{ID: 6, Username: "partner_d", Email: "d@partner.vn", Password: "partner123", Role: "user", Active: true, PortalAccess: false, Prefix: "lm"},
		{ID: 7, Username: "api_bot", Email: "bot@partner.vn", Password: "partner123", Role: "user", Active: false, PortalAccess: false, Prefix: "sale"},
	}
	owners := []struct {
		name   string
		weight float64
		camps  []string
	}{
		{"partner_a", 35, []string{"FPT_INTERNET_T9", "FPT_PLAY_T9", "FPT_CAMERA_Q3", "FPT_SMARTHOME"}},
		{"partner_b", 25, []string{"FPT_PLAY_T9", "FPT_PLAY_T10", "FPT_ID_ONBOARD"}},
		{"partner_c", 20, []string{"FPT_INTERNET_T9", "FPT_INTERNET_T10", "FOXPAY_REF"}},
		{"partner_d", 12, []string{"FPT_TELECOM_B2B"}},
		{"api_bot", 8, []string{"AUTO_SMS"}},
	}
	campNames := map[string]string{
		"FPT_INTERNET_T9":  "Internet cáp quang tháng 9",
		"FPT_INTERNET_T10": "Internet cáp quang tháng 10",
		"FPT_PLAY_T9":      "FPT Play – ưu đãi tháng 9",
		"FPT_PLAY_T10":     "FPT Play – ưu đãi tháng 10",
		"FPT_CAMERA_Q3":    "FPT Camera quý 3",
		"FPT_SMARTHOME":    "FPT Smart Home",
		"FPT_ID_ONBOARD":   "FPT ID – đăng ký mới",
		"FOXPAY_REF":       "Foxpay giới thiệu bạn bè",
		"FPT_TELECOM_B2B":  "Doanh nghiệp B2B",
		"AUTO_SMS":         "SMS tự động",
		"UNUSED_DRAFT":     "Chiến dịch nháp (chưa có link)",
	}
	codes := make([]string, 0, len(campNames))
	for c := range campNames {
		codes = append(codes, c)
	}
	sort.Strings(codes)
	for i, c := range codes {
		ds.Campaigns = append(ds.Campaigns, Campaign{
			ID: int64(i + 1), Code: c, Name: campNames[c], CreatedBy: "admin",
			CreatedAt: from.AddDate(0, 0, -30+i),
		})
	}

	// CTV: mỗi owner có pool riêng; vài CTV dùng chung giữa owner.
	ctvPools := map[string][]string{}
	shared := []string{}
	for i := 0; i < 5; i++ {
		shared = append(shared, randPhone(r))
	}
	seenCTV := map[string]bool{}
	for _, ow := range owners {
		pool := append([]string{}, shared...)
		for i := 0; i < 60; i++ {
			pool = append(pool, randPhone(r))
		}
		for i := 0; i < 6; i++ { // CTV định danh bằng hash
			pool = append(pool, randHex(r, 12))
		}
		ctvPools[ow.name] = pool
		for _, id := range pool {
			if seenCTV[id] {
				continue
			}
			seenCTV[id] = true
			c := CTV{ID: id, Owner: ow.name}
			if mask.IsPhone(id) && r.Float64() < 0.7 {
				c.Name = randName(r)
			}
			ds.CTVs = append(ds.CTVs, c)
		}
	}

	// Link
	ownerW := make([]float64, len(owners))
	for i, ow := range owners {
		ownerW[i] = ow.weight
	}
	usedCodes := map[string]bool{}
	firstLinkDay := from.AddDate(0, 0, -60) // có link tạo trước kỳ click
	span := today.Sub(firstLinkDay).Hours()/24 + 1
	userPrefix := map[string]string{}
	for _, u := range ds.Users {
		userPrefix[u.Username] = u.Prefix
	}
	for i := 0; i < o.Links; i++ {
		ow := owners[pick(r, ownerW)]
		l := Link{ID: int64(100000 + i), Owner: ow.name}
		// ngày tạo: nghiêng về gần đây
		dayOff := int(math.Floor(span * math.Pow(r.Float64(), 0.7)))
		created := firstLinkDay.AddDate(0, 0, dayOff)
		l.CreatedAt = time.Date(created.Year(), created.Month(), created.Day(), 7+r.IntN(15), r.IntN(60), r.IntN(60), 0, VN)
		if l.CreatedAt.After(o.Now) {
			l.CreatedAt = o.Now.Add(-time.Duration(r.IntN(3600)) * time.Second)
		}
		l.APIVersion = pickS(r, []string{"v1", "v2", "v3", "portal"}, []float64{15, 45, 35, 5})
		l.Prefix = userPrefix[ow.name]
		if l.APIVersion == "v3" {
			l.Prefix = "lm" // prefixes.is_default_v3
		}
		l.IsCustom = r.Float64() < 0.05
		for {
			if l.IsCustom {
				l.Code = fmt.Sprintf("%s-%d", strings.ToLower(pickS(r, []string{"km", "uu-dai", "dangky", "play", "net"}, nil)), r.IntN(100000))
			} else {
				l.Code = randBase62(r, 6)
			}
			if !usedCodes[strings.ToLower(l.Code)] {
				usedCodes[strings.ToLower(l.Code)] = true
				break
			}
		}
		if r.Float64() < 0.85 {
			l.Campaign = ow.camps[r.IntN(len(ow.camps))]
		}
		if r.Float64() < 0.88 {
			pool := ctvPools[ow.name]
			l.CTV = pool[int(float64(len(pool))*math.Pow(r.Float64(), 1.6))]
			l.CTVRaw = rawCTV(r, l.CTV)
		}
		l.DestHost = pickS(r, []string{"fpt.vn", "shop.fpt.vn", "fptplay.vn", "id.fpt.vn", "foxpay.vn"}, []float64{40, 20, 20, 10, 10})
		l.LongURL, l.Params = buildLongURL(r, m, l)
		l.Status = "active"
		switch x := r.Float64(); {
		case x < 0.04:
			l.Status = "deleted"
		case x < 0.08:
			l.Status = "disabled"
		}
		if l.Status != "active" {
			l.StatusAt = l.CreatedAt.Add(time.Duration(1+r.IntN(40*24)) * time.Hour)
			if l.StatusAt.After(o.Now) {
				l.Status, l.StatusAt = "active", time.Time{}
			}
		}
		// độ "hot" của link — phân phối đuôi dài
		l.weight = math.Pow(r.ExpFloat64(), 2.2) + 0.02
		if l.CTV == "" {
			l.weight *= 0.6
		}
		ds.Links = append(ds.Links, l)
	}
	sort.Slice(ds.Links, func(i, j int) bool { return ds.Links[i].CreatedAt.Before(ds.Links[j].CreatedAt) })
	for i := range ds.Links {
		ds.Links[i].ID = int64(100000 + i) // id tăng theo thời gian tạo, như counters của Service
	}

	ds.Clicks = genClicks(r, o, ds, from, today)
	return ds
}

func genClicks(r *rand.Rand, o Options, ds *Dataset, from, today time.Time) []Click {
	hourW := []float64{2, 1, 1, 1, 1, 2, 4, 7, 9, 9, 8, 8, 9, 8, 7, 7, 7, 8, 10, 12, 13, 11, 7, 4}
	var clicks []Click

	for d := from; !d.After(today); d = d.AddDate(0, 0, 1) {
		dayStart := time.Date(d.Year(), d.Month(), d.Day(), 0, 0, 0, 0, VN)
		dayEnd := dayStart.Add(24 * time.Hour)
		if dayEnd.After(o.Now) {
			dayEnd = o.Now
		}
		// link nhận click trong ngày: đã tạo và chưa tắt/xoá
		var cand []int
		var w []float64
		for i := range ds.Links {
			l := &ds.Links[i]
			if !l.CreatedAt.Before(dayEnd) {
				break // đã sort theo CreatedAt
			}
			if l.Status != "active" && !l.StatusAt.After(dayStart) {
				continue
			}
			age := dayStart.Sub(l.CreatedAt).Hours() / 24
			decay := 1.0
			if age > 0 {
				decay = math.Exp(-age / 45)
			}
			cand = append(cand, i)
			w = append(w, l.weight*(0.15+decay))
		}
		if len(cand) == 0 {
			continue
		}
		cum := cumulative(w)
		wd := dayStart.Weekday()
		factor := 1.0
		if wd == time.Saturday || wd == time.Sunday {
			factor = 0.75
		}
		frac := dayEnd.Sub(dayStart).Hours() / 24
		n := int(float64(o.ClicksPerDay) * factor * frac * (0.8 + 0.4*r.Float64()))

		pools := map[int64][]Click{}
		for k := 0; k < n; k++ {
			l := &ds.Links[cand[searchCum(cum, r.Float64()*cum[len(cum)-1])]]
			h := pick(r, hourW)
			ts := dayStart.Add(time.Duration(h)*time.Hour + time.Duration(r.IntN(3600))*time.Second)
			if !ts.Before(dayEnd) || ts.Before(l.CreatedAt) || (l.Status != "active" && !ts.Before(l.StatusAt)) {
				continue
			}
			c := makeClick(r, l, ts, randIP(r), r.Float64() < 0.04)
			if p := pools[l.ID]; !c.IsBot && len(p) > 0 && r.Float64() < 0.15 {
				// khách quay lại: cùng IP + thiết bị + vị trí
				v := p[r.IntN(len(p))]
				c.IP, c.Device, c.OS, c.Browser, c.InApp, c.UA = v.IP, v.Device, v.OS, v.Browser, v.InApp, v.UA
				c.Country, c.Province = v.Country, v.Province
			} else if !c.IsBot {
				pools[l.ID] = append(pools[l.ID], c)
			}
			clicks = append(clicks, c)
		}
		// burst: 1 IP bắn 25–45 click vào 1 link trong 1 giờ (để có suspicious_clicks)
		if r.Float64() < 0.35 {
			l := &ds.Links[cand[searchCum(cum, r.Float64()*cum[len(cum)-1])]]
			ip := randIP(r)
			base := dayStart.Add(time.Duration(9+r.IntN(12)) * time.Hour)
			if base.After(l.CreatedAt) && base.Add(time.Hour).Before(dayEnd) && (l.Status == "active" || base.Add(time.Hour).Before(l.StatusAt)) {
				cnt := 25 + r.IntN(21)
				for k := 0; k < cnt; k++ {
					c := makeClick(r, l, base.Add(time.Duration(r.IntN(3600))*time.Second), ip, false)
					c.Device, c.OS, c.Browser, c.InApp = "mobile", "Android", "Chrome", ""
					c.UA = uaFor(c)
					clicks = append(clicks, c)
				}
			}
		}
	}
	sort.SliceStable(clicks, func(i, j int) bool { return clicks[i].TS.Before(clicks[j].TS) })
	flagQuality(clicks)
	for i := range clicks {
		clicks[i].EventID = fmt.Sprintf("seed-%016x", uint64(i)*0x9e3779b97f4a7c15^0xabcdef)
	}
	return clicks
}

// flagQuality: is_repeat (khách đã click link này trong ngày) và is_suspicious (> 20 click / IP / link / giờ).
func flagQuality(clicks []Click) {
	seen := map[string]bool{}
	perHour := map[string]int{}
	for i := range clicks {
		c := &clicks[i]
		if c.IsBot {
			continue
		}
		day := DateOf(c.TS.In(VN)).Format("20060102")
		vk := fmt.Sprintf("%d|%s|%s", c.LinkID, day, VisitorHash(c.IP, c.UA))
		c.IsRepeat = seen[vk]
		seen[vk] = true
		hk := fmt.Sprintf("%d|%s|%s", c.LinkID, c.IP, c.TS.In(VN).Format("2006010215"))
		perHour[hk]++
		c.IsSuspicious = perHour[hk] > 20
	}
}

// VisitorHash: khách = hash(ip + user_agent) (§5.2).
func VisitorHash(ip, ua string) string {
	s := sha256.Sum256([]byte(ip + "|" + ua))
	return hex.EncodeToString(s[:8])
}

func makeClick(r *rand.Rand, l *Link, ts time.Time, ip string, bot bool) Click {
	vn := ts.In(VN)
	c := Click{
		TS: ts.UTC(), LinkID: l.ID, Owner: l.Owner, Campaign: l.Campaign, CTV: l.CTV,
		IP: ip, LinkPrefix: l.Prefix, APIVersion: l.APIVersion, IsCustom: l.IsCustom,
		LinkCreated: l.CreatedAt.UTC(), DestHost: l.DestHost,
		Hour: vn.Hour(), Weekday: int(vn.Weekday()), IsBot: bot,
	}
	c.AccessPrefix = l.Prefix
	if r.Float64() < 0.1 {
		c.AccessPrefix = map[string]string{"sale": "lm", "lm": "sale"}[l.Prefix]
	}
	c.UTM = map[string]string{}
	for _, p := range l.Params {
		if strings.HasPrefix(p.Key, "utm_") && p.Key != "utm_extra_ctv" {
			c.UTM[p.Key] = p.Value
		}
	}
	// nguồn
	c.SourceGroup = pickS(r, []string{"zalo", "facebook", "google", "tiktok", "direct", "other"}, []float64{40, 25, 8, 7, 15, 5})
	switch c.SourceGroup {
	case "zalo":
		c.RefererHost = "zalo.me"
	case "facebook":
		c.RefererHost = pickS(r, []string{"m.facebook.com", "l.facebook.com"}, nil)
	case "google":
		c.RefererHost = "www.google.com"
	case "tiktok":
		c.RefererHost = "www.tiktok.com"
	case "other":
		c.RefererHost = pickS(r, []string{"vnexpress.net", "zingnews.vn", "tinhte.vn"}, nil)
	}
	// vị trí
	c.Country = pickS(r, []string{"VN", "US", "SG", "JP", "KR"}, []float64{96, 1.5, 1.2, 0.8, 0.5})
	if c.Country == "VN" {
		c.Province = pickS(r, provinces, provinceW)
	}
	if bot {
		c.Device = "bot"
		c.OS = "unknown"
		c.Browser = pickS(r, []string{"Googlebot", "facebookexternalhit", "Zalo link preview", "curl"}, nil)
		c.SourceGroup, c.RefererHost = "direct", ""
		c.UA = c.Browser + "/1.0"
		return c
	}
	c.Device = pickS(r, []string{"mobile", "desktop", "tablet", "unknown"}, []float64{78, 16, 4, 2})
	switch c.Device {
	case "mobile":
		c.OS = pickS(r, []string{"Android", "iOS"}, []float64{60, 40})
	case "desktop":
		c.OS = pickS(r, []string{"Windows", "macOS", "Linux"}, []float64{78, 19, 3})
	case "tablet":
		c.OS = pickS(r, []string{"iOS", "Android"}, []float64{65, 35})
	default:
		c.OS = "unknown"
	}
	switch {
	case c.SourceGroup == "zalo" && c.Device != "desktop" && r.Float64() < 0.75:
		c.InApp, c.Browser = "zalo", "Zalo in-app"
	case c.SourceGroup == "facebook" && c.Device != "desktop" && r.Float64() < 0.7:
		c.InApp, c.Browser = "facebook", "Facebook in-app"
	case c.SourceGroup == "tiktok" && c.Device != "desktop" && r.Float64() < 0.8:
		c.InApp, c.Browser = "tiktok", "TikTok in-app"
	case c.OS == "iOS" || c.OS == "macOS":
		c.Browser = pickS(r, []string{"Safari", "Chrome"}, []float64{75, 25})
	case c.OS == "Windows":
		c.Browser = pickS(r, []string{"Chrome", "Cốc Cốc", "Edge", "Firefox"}, []float64{60, 20, 15, 5})
	case c.OS == "unknown":
		c.Browser = "unknown"
	default:
		c.Browser = pickS(r, []string{"Chrome", "Samsung Internet", "Cốc Cốc"}, []float64{75, 15, 10})
	}
	c.UA = uaFor(c)
	return c
}

func uaFor(c Click) string {
	return fmt.Sprintf("Mozilla/5.0 (%s; %s) %s", c.OS, c.Device, c.Browser)
}

var provinces = []string{"Hồ Chí Minh", "Hà Nội", "Đà Nẵng", "Hải Phòng", "Cần Thơ", "Bình Dương", "Đồng Nai", "Khánh Hòa", "Nghệ An", "Thanh Hóa", "Quảng Ninh", "Lâm Đồng"}
var provinceW = []float64{33, 28, 7, 5, 5, 5, 4, 3, 3, 3, 2, 2}

func buildLongURL(r *rand.Rand, m *mask.Masker, l Link) (string, []Param) {
	path := pickS(r, []string{"/internet", "/truyen-hinh", "/khuyen-mai", "/dang-ky", "/camera", ""}, nil)
	q := url.Values{}
	src := pickS(r, []string{"zalo", "facebook", "tiktok", "sms", "email"}, []float64{45, 25, 10, 15, 5})
	q.Set("utm_source", src)
	q.Set("utm_medium", pickS(r, []string{"ctv", "social", "referral"}, []float64{70, 20, 10}))
	if l.Campaign != "" {
		q.Set("utm_campaign", strings.ToLower(l.Campaign))
	}
	if r.Float64() < 0.4 {
		q.Set("utm_content", pickS(r, []string{"banner_a", "banner_b", "video", "post_1", "post_2"}, nil))
	}
	if r.Float64() < 0.1 {
		q.Set("utm_term", pickS(r, []string{"cap quang", "wifi 6", "goi gia dinh"}, nil))
	}
	if l.CTVRaw != "" {
		q.Set("utm_extra_ctv", l.CTVRaw)
	}
	if r.Float64() < 0.15 {
		q.Set("ref", fmt.Sprintf("R%04d", r.IntN(300)))
	}
	if r.Float64() < 0.1 {
		q.Set("promo", pickS(r, []string{"KM50", "FREE1M", "TET2026"}, nil))
	}
	if r.Float64() < 0.05 {
		q.Set("phonenumber", randPhone(r))
	}
	if r.Float64() < 0.03 {
		q.Set("gclid", randHex(r, 24))
	}
	u := "https://" + l.DestHost + path + "?" + q.Encode()

	var ps []Param
	keys := make([]string, 0, len(q))
	for k := range q {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		raw := q.Get(k)
		v := strings.TrimSpace(raw)
		switch k {
		case "utm_source", "utm_medium", "utm_campaign", "promo":
			v = strings.ToLower(v)
		case "utm_extra_ctv":
			v = mask.NormalizePhone(v)
		case "phonenumber":
			// pii = hash: chỉ lưu băm có salt, không lưu bản rõ
			ps = append(ps, Param{Key: k, Value: m.HashPII(mask.NormalizePhone(v))})
			continue
		}
		ps = append(ps, Param{Key: k, Value: v, Raw: raw})
	}
	return u, ps
}

// ---------- tiện ích ngẫu nhiên ----------

func DateOf(t time.Time) time.Time {
	t = t.In(VN)
	return time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, time.UTC)
}

func pick(r *rand.Rand, w []float64) int {
	cum := cumulative(w)
	return searchCum(cum, r.Float64()*cum[len(cum)-1])
}

func pickS(r *rand.Rand, opts []string, w []float64) string {
	if w == nil {
		return opts[r.IntN(len(opts))]
	}
	return opts[pick(r, w)]
}

func cumulative(w []float64) []float64 {
	out := make([]float64, len(w))
	s := 0.0
	for i, v := range w {
		s += v
		out[i] = s
	}
	return out
}

func searchCum(cum []float64, x float64) int {
	i := sort.SearchFloat64s(cum, x)
	if i >= len(cum) {
		i = len(cum) - 1
	}
	return i
}

func randPhone(r *rand.Rand) string {
	pre := []string{"090", "091", "093", "094", "096", "097", "098", "086", "088", "070", "077", "079", "081", "083", "085", "032", "033", "035", "037", "039"}
	return pre[r.IntN(len(pre))] + fmt.Sprintf("%07d", r.IntN(10_000_000))
}

// rawCTV: CTV nhập tay nhiều định dạng (để test chuẩn hoá).
func rawCTV(r *rand.Rand, id string) string {
	if !mask.IsPhone(id) {
		return id
	}
	switch r.IntN(4) {
	case 0:
		return "84" + id[1:]
	case 1:
		return id[:4] + " " + id[4:7] + " " + id[7:]
	default:
		return id
	}
}

func randHex(r *rand.Rand, n int) string {
	const h = "0123456789abcdef"
	b := make([]byte, n)
	for i := range b {
		b[i] = h[r.IntN(16)]
	}
	return string(b)
}

func randBase62(r *rand.Rand, n int) string {
	const a = "0123456789abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ"
	b := make([]byte, n)
	for i := range b {
		b[i] = a[r.IntN(len(a))]
	}
	return string(b)
}

func randIP(r *rand.Rand) string {
	pre := [][2]int{{113, 161}, {113, 185}, {27, 72}, {14, 161}, {171, 224}, {42, 112}, {116, 110}, {1, 52}, {58, 186}, {123, 20}}
	p := pre[r.IntN(len(pre))]
	return fmt.Sprintf("%d.%d.%d.%d", p[0], p[1], r.IntN(256), 1+r.IntN(254))
}

func randName(r *rand.Rand) string {
	ho := []string{"Nguyễn", "Trần", "Lê", "Phạm", "Hoàng", "Huỳnh", "Phan", "Vũ", "Võ", "Đặng", "Bùi", "Đỗ"}
	dem := []string{"Văn", "Thị", "Minh", "Thanh", "Ngọc", "Đức", "Hữu", "Thu", "Quốc", "Gia"}
	ten := []string{"An", "Bình", "Châu", "Dũng", "Giang", "Hà", "Hùng", "Khoa", "Lan", "Long", "Mai", "Nam", "Phúc", "Quân", "Sơn", "Trang", "Tuấn", "Vy"}
	return ho[r.IntN(len(ho))] + " " + dem[r.IntN(len(dem))] + " " + ten[r.IntN(len(ten))]
}

package export

import (
	"bufio"
	"context"
	"encoding/csv"
	"encoding/json"
	"fmt"
	"os"
	"strconv"
	"time"

	"github.com/xuri/excelize/v2"

	"am-shortlink-portal/api/internal/domain"
	"am-shortlink-portal/api/internal/httpapi/gen"
	"am-shortlink-portal/api/internal/report"
	"am-shortlink-portal/api/internal/store"
)

// rowWriter: CSV hoặc XLSX streaming.
type rowWriter interface {
	Row(vals ...any) error
	Close() error
}

type csvWriter struct {
	f *os.File
	b *bufio.Writer
	w *csv.Writer
}

func newCSV(path string) (*csvWriter, error) {
	f, err := os.Create(path)
	if err != nil {
		return nil, err
	}
	b := bufio.NewWriterSize(f, 1<<16)
	_, _ = b.Write([]byte{0xEF, 0xBB, 0xBF}) // BOM UTF-8 để Excel mở đúng tiếng Việt
	return &csvWriter{f: f, b: b, w: csv.NewWriter(b)}, nil
}

func (c *csvWriter) Row(vals ...any) error {
	rec := make([]string, len(vals))
	for i, v := range vals {
		rec[i] = cell(v)
	}
	return c.w.Write(rec)
}

func (c *csvWriter) Close() error {
	c.w.Flush()
	if err := c.w.Error(); err != nil {
		c.f.Close()
		return err
	}
	if err := c.b.Flush(); err != nil {
		c.f.Close()
		return err
	}
	return c.f.Close()
}

type xlsxWriter struct {
	f    *excelize.File
	sw   *excelize.StreamWriter
	row  int
	path string
}

func newXLSX(path string) (*xlsxWriter, error) {
	f := excelize.NewFile()
	sw, err := f.NewStreamWriter("Sheet1")
	if err != nil {
		return nil, err
	}
	return &xlsxWriter{f: f, sw: sw, row: 1, path: path}, nil
}

func (x *xlsxWriter) Row(vals ...any) error {
	cells := make([]any, len(vals))
	for i, v := range vals {
		switch t := v.(type) {
		case int64, int, float64:
			cells[i] = t
		case *float64:
			if t != nil {
				cells[i] = *t
			}
		default:
			cells[i] = cell(v)
		}
	}
	ref, _ := excelize.CoordinatesToCellName(1, x.row)
	x.row++
	return x.sw.SetRow(ref, cells)
}

func (x *xlsxWriter) Close() error {
	if err := x.sw.Flush(); err != nil {
		return err
	}
	if err := x.f.SaveAs(x.path); err != nil {
		return err
	}
	return x.f.Close()
}

func cell(v any) string {
	switch t := v.(type) {
	case nil:
		return ""
	case string:
		return t
	case *string:
		if t == nil {
			return ""
		}
		return *t
	case int64:
		return strconv.FormatInt(t, 10)
	case int:
		return strconv.Itoa(t)
	case float64:
		return strconv.FormatFloat(t, 'f', 4, 64)
	case *float64:
		if t == nil {
			return ""
		}
		return strconv.FormatFloat(*t, 'f', 4, 64)
	case time.Time:
		return t.In(store.VN).Format("2006-01-02 15:04:05")
	case bool:
		if t {
			return "1"
		}
		return "0"
	}
	return fmt.Sprint(v)
}

// ---------- tham số job → report.Query ----------

type params struct {
	From, To    string
	Account     []string
	Campaign    []string
	Ctv         []string
	Prefix      []string
	Compare     bool
	Granularity string
	Mode        string
	DeadDays    int
	Filters     *gen.ClickFilters
	Explorer    *gen.ExplorerQuery
}

func decodeParams(raw map[string]any) (params, error) {
	b, err := json.Marshal(raw)
	if err != nil {
		return params{}, err
	}
	var p struct {
		From        string            `json:"from"`
		To          string            `json:"to"`
		Account     []string          `json:"account"`
		Campaign    []string          `json:"campaign"`
		Ctv         []string          `json:"ctv"`
		Prefix      []string          `json:"prefix"`
		Compare     bool              `json:"compare"`
		Granularity string            `json:"granularity"`
		Mode        string            `json:"mode"`
		DeadDays    int               `json:"dead_days"`
		Filters     *gen.ClickFilters `json:"filters"`
	}
	if err := json.Unmarshal(b, &p); err != nil {
		return params{}, domain.BadRequest("invalid_export_params", "tham số xuất dữ liệu không hợp lệ")
	}
	out := params{From: p.From, To: p.To, Account: p.Account, Campaign: p.Campaign, Ctv: p.Ctv, Prefix: p.Prefix,
		Compare: p.Compare, Granularity: p.Granularity, Mode: p.Mode, DeadDays: p.DeadDays, Filters: p.Filters}
	var ex gen.ExplorerQuery
	if err := json.Unmarshal(b, &ex); err == nil && len(ex.GroupBy) > 0 {
		out.Explorer = &ex
	}
	return out, nil
}

func (s *Service) query(p domain.Principal, raw map[string]any) (report.Query, error) {
	pp, err := decodeParams(raw)
	if err != nil {
		return report.Query{}, err
	}
	from, err1 := time.Parse("2006-01-02", pp.From)
	to, err2 := time.Parse("2006-01-02", pp.To)
	if err1 != nil || err2 != nil {
		return report.Query{}, domain.BadRequest("invalid_export_params", "thiếu / sai from, to (yyyy-mm-dd)")
	}
	g := store.Granularity(pp.Granularity)
	if g == "" {
		g = store.Day
	}
	return s.reports.Build(p, report.Params{From: from, To: to, Accounts: pp.Account, Campaigns: pp.Campaign, CTVs: pp.Ctv, Prefixes: pp.Prefix, Compare: pp.Compare, Granularity: g})
}

func (s *Service) write(ctx context.Context, j *store.ExportJob, path string) (int64, error) {
	p := principalOf(j)
	q, err := s.query(p, j.Params)
	if err != nil {
		return 0, err
	}
	pp, _ := decodeParams(j.Params)
	var w rowWriter
	if j.Format == "xlsx" {
		w, err = newXLSX(path)
	} else {
		w, err = newCSV(path)
	}
	if err != nil {
		return 0, err
	}
	var n int64
	add := func(vals ...any) error {
		n++
		if n > MaxRows {
			return domain.Unprocessable("too_many_rows", "vượt 1 triệu dòng — hãy thu hẹp bộ lọc")
		}
		return w.Row(vals...)
	}
	all := report.Paging{Page: 1, PageSize: MaxRows, Desc: true}
	err = func() error {
		switch j.Kind {
		case "accounts":
			r, err := s.reports.Accounts(ctx, q, all)
			if err != nil {
				return err
			}
			_ = w.Row("Tài khoản", "Tổng link", "Link mới", "Link có click", "Lượt click", "Khách duy nhất", "Bot", "Nghi vấn", "Click/link", "So kỳ trước")
			for _, it := range r.Items {
				m := it.Metrics
				if err := add(it.Username, m.TotalLinks, m.NewLinks, m.ActiveLinks, m.Clicks, m.UniqueClicks, m.BotClicks, m.SuspiciousClicks, m.CtrPerLink, it.ChangeClicks); err != nil {
					return err
				}
			}
		case "campaigns":
			r, err := s.reports.Campaigns(ctx, q, all)
			if err != nil {
				return err
			}
			_ = w.Row("Mã chiến dịch", "Tên", "Link mới", "Link có click", "Lượt click", "Khách duy nhất", "Bot", "Nghi vấn", "Số CTV", "Ngày bắt đầu", "Click cuối")
			for _, it := range r.Items {
				m := it.Metrics
				if err := add(it.Code, it.Name, m.NewLinks, m.ActiveLinks, m.Clicks, m.UniqueClicks, m.BotClicks, m.SuspiciousClicks, it.CtvCount, dateStr(it.FirstDate), dateStr(it.LastClickDate)); err != nil {
					return err
				}
			}
		case "ctvs":
			r, err := s.reports.CTVs(ctx, q, report.Paging{Page: 1, PageSize: MaxRows})
			if err != nil {
				return err
			}
			_ = w.Row("Hạng", "CTV", "Tên", "Link mới", "Link có click", "Lượt click", "Khách duy nhất", "Nghi vấn", "Click/link")
			for _, it := range r.Items {
				m := it.Metrics
				if err := add(it.Rank, it.CtvDisplay, it.Name, m.NewLinks, m.ActiveLinks, m.Clicks, m.UniqueClicks, m.SuspiciousClicks, m.CtrPerLink); err != nil {
					return err
				}
			}
		case "links_top":
			r, err := s.reports.TopLinksPage(ctx, q, pp.Mode, pp.DeadDays, report.Paging{Page: 1, PageSize: MaxRows})
			if err != nil {
				return err
			}
			_ = w.Row("Short URL", "Long URL", "Tài khoản", "Chiến dịch", "CTV", "Trạng thái", "Lượt click", "Khách", "Kỳ trước", "Tăng trưởng", "Click cuối", "Ngày tạo")
			for _, it := range r.Items {
				l := it.Link
				if err := add(l.ShortUrl, l.LongUrl, l.Owner, l.CampaignCode, l.CtvDisplay, string(l.Status), it.Clicks, it.UniqueClicks, it.PreviousClicks, it.Growth, dateStr(it.LastClickDate), l.CreatedAt); err != nil {
					return err
				}
			}
		case "explorer":
			if pp.Explorer == nil {
				return domain.BadRequest("invalid_export_params", "thiếu group_by cho xuất Explorer")
			}
			ex := *pp.Explorer
			lim := 1000
			ex.Limit = &lim
			r, err := s.reports.Explorer(ctx, q, ex)
			if err != nil {
				return err
			}
			head := append([]any{}, toAny(r.GroupBy)...)
			head = append(head, "Lượt click", "% tổng", "Khách", "Bot", "Nghi vấn", "Link có click", "Click/link", "So kỳ trước")
			_ = w.Row(head...)
			rows := r.Rows
			if r.Other != nil {
				rows = append(rows, *r.Other)
			}
			for _, row := range rows {
				vals := make([]any, 0, len(r.GroupBy)+8)
				for i := range r.GroupBy {
					if i < len(row.Keys) {
						vals = append(vals, row.Keys[i].Label)
					} else {
						vals = append(vals, "")
					}
				}
				m := row.Metrics
				vals = append(vals, m.Clicks, row.Share, m.UniqueClicks, m.BotClicks, m.SuspiciousClicks, m.ActiveLinks, m.ClicksPerLink, row.ChangeClicks)
				if err := add(vals...); err != nil {
					return err
				}
			}
		case "clicks":
			_ = w.Row("Thời gian", "Link", "Prefix truy cập", "Long URL", "Tài khoản", "Chiến dịch", "CTV", "Thiết bị", "HĐH", "Trình duyệt", "Nguồn", "Referer", "Quốc gia", "Tỉnh/thành", "IP", "Bot", "Nghi vấn", "Lặp lại")
			cursor := ""
			for {
				page, err := s.reports.Clicks(ctx, q, pp.Filters, cursor, 200)
				if err != nil {
					return err
				}
				for _, c := range page.Items {
					if err := add(c.Ts, c.Code, c.AccessPrefix, c.LongUrl, c.Owner, c.CampaignCode, c.CtvDisplay, c.Device, c.Os, c.Browser, c.SourceGroup, c.RefererHost, c.Country, c.Province, c.Ip, c.IsBot, c.IsSuspicious, c.IsRepeat); err != nil {
						return err
					}
				}
				_ = s.st.HeartbeatExport(ctx, j.ID, n, s.now().UTC())
				if page.NextCursor == nil {
					return nil
				}
				cursor = *page.NextCursor
			}
		default:
			return domain.BadRequest("invalid_export_kind", "loại xuất không hỗ trợ: "+j.Kind)
		}
		return nil
	}()
	if cerr := w.Close(); err == nil {
		err = cerr
	}
	return n, err
}

func dateStr(d interface{ String() string }) string {
	if d == nil {
		return ""
	}
	return d.String()
}

func toAny(ss []string) []any {
	out := make([]any, len(ss))
	for i, s := range ss {
		out[i] = s
	}
	return out
}

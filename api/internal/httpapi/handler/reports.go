package handler

import (
	"context"

	openapi_types "github.com/oapi-codegen/runtime/types"

	"am-shortlink-portal/api/internal/httpapi/gen"
	"am-shortlink-portal/api/internal/report"
	"am-shortlink-portal/api/internal/store"
)

// filterArgs: các tham số lọc chung (con trỏ = không truyền).
type filterArgs struct {
	From, To    openapi_types.Date
	Account     *[]string
	Campaign    *[]string
	Ctv         *[]string
	Prefix      *[]string
	Compare     *bool
	Granularity *gen.Granularity
}

func (h *Handler) query(ctx context.Context, a filterArgs) (report.Query, error) {
	p, err := principal(ctx)
	if err != nil {
		return report.Query{}, err
	}
	g := store.Day
	if a.Granularity != nil {
		g = store.Granularity(*a.Granularity)
	}
	return h.Reports.Build(p, report.Params{
		From: a.From.Time, To: a.To.Time,
		Accounts: deref(a.Account, nil), Campaigns: deref(a.Campaign, nil),
		CTVs: deref(a.Ctv, nil), Prefixes: deref(a.Prefix, nil),
		Compare: deref(a.Compare, false), Granularity: g,
	})
}

func paging(q *string, page, size *int, sort *string, order *gen.SortOrder) report.Paging {
	return report.Paging{
		Q: deref(q, ""), Page: deref(page, 1), PageSize: deref(size, 50), Sort: deref(sort, ""),
		Desc: order == nil || *order == gen.SortOrder("desc"),
	}
}

func (h *Handler) GetOverview(ctx context.Context, r gen.GetOverviewRequestObject) (gen.GetOverviewResponseObject, error) {
	p := r.Params
	q, err := h.query(ctx, filterArgs{p.From, p.To, p.Account, p.Campaign, p.Ctv, p.Prefix, p.Compare, p.Granularity})
	if err != nil {
		return nil, err
	}
	out, err := h.Reports.Overview(ctx, q)
	if err != nil {
		return nil, err
	}
	return gen.GetOverview200JSONResponse(out), nil
}

func (h *Handler) ListAccountReports(ctx context.Context, r gen.ListAccountReportsRequestObject) (gen.ListAccountReportsResponseObject, error) {
	p := r.Params
	q, err := h.query(ctx, filterArgs{p.From, p.To, p.Account, p.Campaign, p.Ctv, p.Prefix, p.Compare, nil})
	if err != nil {
		return nil, err
	}
	out, err := h.Reports.Accounts(ctx, q, paging(p.Q, p.Page, p.PageSize, p.Sort, p.Order))
	if err != nil {
		return nil, err
	}
	return gen.ListAccountReports200JSONResponse(out), nil
}

func (h *Handler) GetAccountReport(ctx context.Context, r gen.GetAccountReportRequestObject) (gen.GetAccountReportResponseObject, error) {
	p := r.Params
	q, err := h.query(ctx, filterArgs{p.From, p.To, nil, p.Campaign, p.Ctv, p.Prefix, p.Compare, p.Granularity})
	if err != nil {
		return nil, err
	}
	out, err := h.Reports.Account(ctx, q, r.Username)
	if err != nil {
		return nil, err
	}
	return gen.GetAccountReport200JSONResponse(out), nil
}

func (h *Handler) ListCampaignReports(ctx context.Context, r gen.ListCampaignReportsRequestObject) (gen.ListCampaignReportsResponseObject, error) {
	p := r.Params
	q, err := h.query(ctx, filterArgs{p.From, p.To, p.Account, p.Campaign, p.Ctv, p.Prefix, p.Compare, nil})
	if err != nil {
		return nil, err
	}
	out, err := h.Reports.Campaigns(ctx, q, paging(p.Q, p.Page, p.PageSize, p.Sort, p.Order))
	if err != nil {
		return nil, err
	}
	return gen.ListCampaignReports200JSONResponse(out), nil
}

func (h *Handler) GetCampaignReport(ctx context.Context, r gen.GetCampaignReportRequestObject) (gen.GetCampaignReportResponseObject, error) {
	p := r.Params
	q, err := h.query(ctx, filterArgs{p.From, p.To, p.Account, nil, p.Ctv, p.Prefix, p.Compare, p.Granularity})
	if err != nil {
		return nil, err
	}
	out, err := h.Reports.Campaign(ctx, q, r.Code)
	if err != nil {
		return nil, err
	}
	return gen.GetCampaignReport200JSONResponse(out), nil
}

func (h *Handler) CompareCampaigns(ctx context.Context, r gen.CompareCampaignsRequestObject) (gen.CompareCampaignsResponseObject, error) {
	p := r.Params
	q, err := h.query(ctx, filterArgs{p.From, p.To, p.Account, nil, nil, nil, nil, p.Granularity})
	if err != nil {
		return nil, err
	}
	out, err := h.Reports.Compare(ctx, q, p.Codes)
	if err != nil {
		return nil, err
	}
	return gen.CompareCampaigns200JSONResponse(out), nil
}

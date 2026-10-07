package handler

import (
	"context"

	"am-shortlink-portal/api/internal/httpapi/gen"
	"am-shortlink-portal/api/internal/report"
	"am-shortlink-portal/api/internal/store"
)

func (h *Handler) QueryClicks(ctx context.Context, r gen.QueryClicksRequestObject) (gen.QueryClicksResponseObject, error) {
	b := *r.Body
	p, err := principal(ctx)
	if err != nil {
		return nil, err
	}
	q, err := h.Reports.Build(p, report.Params{
		From: b.From.Time, To: b.To.Time, Accounts: deref(b.Account, nil), Campaigns: deref(b.Campaign, nil),
		CTVs: deref(b.Ctv, nil), Prefixes: deref(b.Prefix, nil), Compare: deref(b.Compare, false), Granularity: store.Day,
	})
	if err != nil {
		return nil, err
	}
	out, err := h.Reports.Explorer(ctx, q, b)
	if err != nil {
		return nil, err
	}
	return gen.QueryClicks200JSONResponse(out), nil
}

func (h *Handler) GetClickFacets(ctx context.Context, r gen.GetClickFacetsRequestObject) (gen.GetClickFacetsResponseObject, error) {
	p, err := principal(ctx)
	if err != nil {
		return nil, err
	}
	q, err := h.Reports.ScopeOnly(p, deref(r.Params.Account, nil))
	if err != nil {
		return nil, err
	}
	out, err := h.Reports.Facets(ctx, q, r.Params.Field, deref(r.Params.Q, ""), deref(r.Params.Limit, 100))
	if err != nil {
		return nil, err
	}
	return gen.GetClickFacets200JSONResponse(out), nil
}

func (h *Handler) ListParamReports(ctx context.Context, r gen.ListParamReportsRequestObject) (gen.ListParamReportsResponseObject, error) {
	p := r.Params
	q, err := h.query(ctx, filterArgs{p.From, p.To, p.Account, p.Campaign, nil, nil, nil, nil})
	if err != nil {
		return nil, err
	}
	out, err := h.Reports.Params(ctx, q)
	if err != nil {
		return nil, err
	}
	return gen.ListParamReports200JSONResponse(out), nil
}

func (h *Handler) GetParamReport(ctx context.Context, r gen.GetParamReportRequestObject) (gen.GetParamReportResponseObject, error) {
	p := r.Params
	q, err := h.query(ctx, filterArgs{p.From, p.To, p.Account, p.Campaign, nil, nil, p.Compare, p.Granularity})
	if err != nil {
		return nil, err
	}
	out, err := h.Reports.Param(ctx, q, r.Key, paging(p.Q, p.Page, p.PageSize, p.Sort, p.Order))
	if err != nil {
		return nil, err
	}
	return gen.GetParamReport200JSONResponse(out), nil
}

// ---- báo cáo đã lưu ----

func (h *Handler) ListSavedReports(ctx context.Context, _ gen.ListSavedReportsRequestObject) (gen.ListSavedReportsResponseObject, error) {
	p, err := principal(ctx)
	if err != nil {
		return nil, err
	}
	out, err := h.Saved.List(ctx, p)
	if err != nil {
		return nil, err
	}
	return gen.ListSavedReports200JSONResponse(out), nil
}

func (h *Handler) CreateSavedReport(ctx context.Context, r gen.CreateSavedReportRequestObject) (gen.CreateSavedReportResponseObject, error) {
	p, err := principal(ctx)
	if err != nil {
		return nil, err
	}
	out, err := h.Saved.Create(ctx, p, *r.Body)
	if err != nil {
		return nil, err
	}
	return gen.CreateSavedReport201JSONResponse(out), nil
}

func (h *Handler) GetSharedReport(ctx context.Context, r gen.GetSharedReportRequestObject) (gen.GetSharedReportResponseObject, error) {
	p, err := principal(ctx)
	if err != nil {
		return nil, err
	}
	out, err := h.Saved.GetShared(ctx, p, r.Token)
	if err != nil {
		return nil, err
	}
	return gen.GetSharedReport200JSONResponse(out), nil
}

func (h *Handler) GetSavedReport(ctx context.Context, r gen.GetSavedReportRequestObject) (gen.GetSavedReportResponseObject, error) {
	p, err := principal(ctx)
	if err != nil {
		return nil, err
	}
	out, err := h.Saved.Get(ctx, p, r.Id)
	if err != nil {
		return nil, err
	}
	return gen.GetSavedReport200JSONResponse(out), nil
}

func (h *Handler) UpdateSavedReport(ctx context.Context, r gen.UpdateSavedReportRequestObject) (gen.UpdateSavedReportResponseObject, error) {
	p, err := principal(ctx)
	if err != nil {
		return nil, err
	}
	out, err := h.Saved.Update(ctx, p, r.Id, *r.Body)
	if err != nil {
		return nil, err
	}
	return gen.UpdateSavedReport200JSONResponse(out), nil
}

func (h *Handler) DeleteSavedReport(ctx context.Context, r gen.DeleteSavedReportRequestObject) (gen.DeleteSavedReportResponseObject, error) {
	p, err := principal(ctx)
	if err != nil {
		return nil, err
	}
	if err := h.Saved.Delete(ctx, p, r.Id); err != nil {
		return nil, err
	}
	return gen.DeleteSavedReport204Response{}, nil
}

func (h *Handler) ShareSavedReport(ctx context.Context, r gen.ShareSavedReportRequestObject) (gen.ShareSavedReportResponseObject, error) {
	p, err := principal(ctx)
	if err != nil {
		return nil, err
	}
	out, err := h.Saved.Share(ctx, p, r.Id, r.Body.Enabled)
	if err != nil {
		return nil, err
	}
	return gen.ShareSavedReport200JSONResponse(out), nil
}

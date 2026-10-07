package handler

import (
	"bytes"
	"context"

	"am-shortlink-portal/api/internal/httpapi/gen"
	"am-shortlink-portal/api/internal/report"
)

func (h *Handler) ListCtvReports(ctx context.Context, r gen.ListCtvReportsRequestObject) (gen.ListCtvReportsResponseObject, error) {
	p := r.Params
	q, err := h.query(ctx, filterArgs{p.From, p.To, p.Account, p.Campaign, p.Ctv, p.Prefix, p.Compare, nil})
	if err != nil {
		return nil, err
	}
	out, err := h.Reports.CTVs(ctx, q, paging(p.Q, p.Page, p.PageSize, p.Sort, p.Order))
	if err != nil {
		return nil, err
	}
	return gen.ListCtvReports200JSONResponse(out), nil
}

func (h *Handler) GetCtvReport(ctx context.Context, r gen.GetCtvReportRequestObject) (gen.GetCtvReportResponseObject, error) {
	p := r.Params
	q, err := h.query(ctx, filterArgs{p.From, p.To, p.Account, p.Campaign, nil, p.Prefix, p.Compare, p.Granularity})
	if err != nil {
		return nil, err
	}
	out, err := h.Reports.CTV(ctx, q, r.Ref)
	if err != nil {
		return nil, err
	}
	return gen.GetCtvReport200JSONResponse(out), nil
}

func (h *Handler) LookupCtv(ctx context.Context, r gen.LookupCtvRequestObject) (gen.LookupCtvResponseObject, error) {
	p, err := principal(ctx)
	if err != nil {
		return nil, err
	}
	out, err := h.Reports.Lookup(ctx, p, r.Params.Q)
	if err != nil {
		return nil, err
	}
	return gen.LookupCtv200JSONResponse(out), nil
}

func (h *Handler) GetUnidentifiedCtv(ctx context.Context, r gen.GetUnidentifiedCtvRequestObject) (gen.GetUnidentifiedCtvResponseObject, error) {
	p := r.Params
	q, err := h.query(ctx, filterArgs{p.From, p.To, p.Account, p.Campaign, nil, p.Prefix, nil, nil})
	if err != nil {
		return nil, err
	}
	out, err := h.Reports.Unidentified(ctx, q, report.Paging{Page: deref(p.Page, 1), PageSize: deref(p.PageSize, 50)})
	if err != nil {
		return nil, err
	}
	return gen.GetUnidentifiedCtv200JSONResponse(out), nil
}

func (h *Handler) ListTopLinks(ctx context.Context, r gen.ListTopLinksRequestObject) (gen.ListTopLinksResponseObject, error) {
	p := r.Params
	q, err := h.query(ctx, filterArgs{p.From, p.To, p.Account, p.Campaign, p.Ctv, p.Prefix, nil, nil})
	if err != nil {
		return nil, err
	}
	mode := "clicks"
	if p.Mode != nil {
		mode = string(*p.Mode)
	}
	out, err := h.Reports.TopLinksPage(ctx, q, mode, deref(p.DeadDays, 14), report.Paging{Page: deref(p.Page, 1), PageSize: deref(p.PageSize, 50)})
	if err != nil {
		return nil, err
	}
	return gen.ListTopLinks200JSONResponse(out), nil
}

func (h *Handler) GetLinkReport(ctx context.Context, r gen.GetLinkReportRequestObject) (gen.GetLinkReportResponseObject, error) {
	p := r.Params
	q, err := h.query(ctx, filterArgs{p.From, p.To, nil, nil, nil, nil, p.Compare, p.Granularity})
	if err != nil {
		return nil, err
	}
	out, err := h.Reports.Link(ctx, q, r.Code)
	if err != nil {
		return nil, err
	}
	return gen.GetLinkReport200JSONResponse(out), nil
}

func (h *Handler) GetLinkQrcode(ctx context.Context, r gen.GetLinkQrcodeRequestObject) (gen.GetLinkQrcodeResponseObject, error) {
	p, err := principal(ctx)
	if err != nil {
		return nil, err
	}
	png, err := h.Reports.QRCode(ctx, p, r.Code, deref(r.Params.Size, 256))
	if err != nil {
		return nil, err
	}
	return gen.GetLinkQrcode200ImagepngResponse{Body: bytes.NewReader(png), ContentLength: int64(len(png))}, nil
}

func (h *Handler) ListClicks(ctx context.Context, r gen.ListClicksRequestObject) (gen.ListClicksResponseObject, error) {
	p := r.Params
	q, err := h.query(ctx, filterArgs{p.From, p.To, p.Account, p.Campaign, p.Ctv, p.Prefix, nil, nil})
	if err != nil {
		return nil, err
	}
	out, err := h.Reports.Clicks(ctx, q, p.Filters, deref(p.Cursor, ""), deref(p.PageSize, 50))
	if err != nil {
		return nil, err
	}
	return gen.ListClicks200JSONResponse(out), nil
}

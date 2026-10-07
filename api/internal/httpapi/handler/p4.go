package handler

import (
	"context"
	"strconv"

	"am-shortlink-portal/api/internal/httpapi/gen"
)

func (h *Handler) GetTrafficQuality(ctx context.Context, r gen.GetTrafficQualityRequestObject) (gen.GetTrafficQualityResponseObject, error) {
	p := r.Params
	q, err := h.query(ctx, filterArgs{p.From, p.To, p.Account, p.Campaign, p.Ctv, p.Prefix, nil, nil})
	if err != nil {
		return nil, err
	}
	out, err := h.Reports.TrafficQuality(ctx, q)
	if err != nil {
		return nil, err
	}
	return gen.GetTrafficQuality200JSONResponse(out), nil
}

// ---- export ----

func (h *Handler) ListExports(ctx context.Context, _ gen.ListExportsRequestObject) (gen.ListExportsResponseObject, error) {
	p, err := principal(ctx)
	if err != nil {
		return nil, err
	}
	out, err := h.Exports.List(ctx, p)
	if err != nil {
		return nil, err
	}
	return gen.ListExports200JSONResponse(out), nil
}

func (h *Handler) CreateExport(ctx context.Context, r gen.CreateExportRequestObject) (gen.CreateExportResponseObject, error) {
	p, err := principal(ctx)
	if err != nil {
		return nil, err
	}
	out, err := h.Exports.Create(ctx, p, *r.Body)
	if err != nil {
		return nil, err
	}
	return gen.CreateExport202JSONResponse(out), nil
}

func (h *Handler) GetExport(ctx context.Context, r gen.GetExportRequestObject) (gen.GetExportResponseObject, error) {
	p, err := principal(ctx)
	if err != nil {
		return nil, err
	}
	out, err := h.Exports.Get(ctx, p, r.Id)
	if err != nil {
		return nil, err
	}
	return gen.GetExport200JSONResponse(out), nil
}

func (h *Handler) DownloadExport(ctx context.Context, r gen.DownloadExportRequestObject) (gen.DownloadExportResponseObject, error) {
	p, err := principal(ctx)
	if err != nil {
		return nil, err
	}
	f, size, name, err := h.Exports.Download(ctx, p, r.Id)
	if err != nil {
		return nil, err
	}
	// *os.File là io.ReadCloser → strict handler đóng sau khi gửi xong.
	cd := "attachment; filename=" + strconv.Quote(name)
	return gen.DownloadExport200ApplicationoctetStreamResponse{Body: f, ContentLength: size, Headers: gen.DownloadExport200ResponseHeaders{ContentDisposition: &cd}}, nil
}

// ---- quản lý tham số ----

func (h *Handler) ListParamRegistry(ctx context.Context, _ gen.ListParamRegistryRequestObject) (gen.ListParamRegistryResponseObject, error) {
	p, err := principal(ctx)
	if err != nil {
		return nil, err
	}
	out, err := h.Reports.Registry(ctx, p)
	if err != nil {
		return nil, err
	}
	return gen.ListParamRegistry200JSONResponse(out), nil
}

func (h *Handler) UpdateParamRegistry(ctx context.Context, r gen.UpdateParamRegistryRequestObject) (gen.UpdateParamRegistryResponseObject, error) {
	p, err := principal(ctx)
	if err != nil {
		return nil, err
	}
	out, err := h.Reports.UpdateRegistry(ctx, p, r.Key, *r.Body)
	if err != nil {
		return nil, err
	}
	return gen.UpdateParamRegistry200JSONResponse(out), nil
}

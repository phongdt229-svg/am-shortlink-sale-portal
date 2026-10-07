package handler

import (
	"context"

	"am-shortlink-portal/api/internal/httpapi/gen"
)

func (h *Handler) ListFilterAccounts(ctx context.Context, req gen.ListFilterAccountsRequestObject) (gen.ListFilterAccountsResponseObject, error) {
	_, sc, err := scopeOf(ctx)
	if err != nil {
		return nil, err
	}
	owners, unrestricted, _ := sc.Resolve(nil)
	names, err := h.Store.ListUsernames(ctx, deref(req.Params.Q, ""), deref(req.Params.Limit, 50), owners, unrestricted)
	if err != nil {
		return nil, err
	}
	items := make([]gen.AccountOption, len(names))
	for i, n := range names {
		items[i] = gen.AccountOption{Username: n}
	}
	return gen.ListFilterAccounts200JSONResponse{Items: items}, nil
}

func (h *Handler) ListFilterCampaigns(ctx context.Context, req gen.ListFilterCampaignsRequestObject) (gen.ListFilterCampaignsResponseObject, error) {
	_, sc, err := scopeOf(ctx)
	if err != nil {
		return nil, err
	}
	ownerFilter, err := sc.Filter("owner", deref(req.Params.Account, nil))
	if err != nil {
		return nil, err
	}
	list, err := h.Store.ListCampaigns(ctx, deref(req.Params.Q, ""), deref(req.Params.Limit, 50), ownerFilter)
	if err != nil {
		return nil, err
	}
	items := make([]gen.CampaignOption, len(list))
	for i, c := range list {
		items[i] = gen.CampaignOption{Code: c.Code, Name: c.Name}
	}
	return gen.ListFilterCampaigns200JSONResponse{Items: items}, nil
}

func (h *Handler) ListFilterPrefixes(ctx context.Context, _ gen.ListFilterPrefixesRequestObject) (gen.ListFilterPrefixesResponseObject, error) {
	if _, err := principal(ctx); err != nil {
		return nil, err
	}
	list, err := h.Store.ListPrefixes(ctx)
	if err != nil {
		return nil, err
	}
	items := make([]gen.PrefixOption, len(list))
	for i, p := range list {
		items[i] = gen.PrefixOption{Id: p.ID, IsDefault: p.IsDefault}
		if p.Description != "" {
			d := p.Description
			items[i].Description = &d
		}
	}
	return gen.ListFilterPrefixes200JSONResponse{Items: items}, nil
}

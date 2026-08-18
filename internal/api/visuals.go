package api

import (
	"context"
	"errors"
	"net/url"
	"strconv"
	"strings"
)

func (c *Client) ListVisuals(ctx context.Context, category VisualCategory, page, pageSize int) (*VisualsData, error) {
	category = VisualCategory(strings.ToLower(strings.TrimSpace(string(category))))
	switch category {
	case VisualCategoryImages, VisualCategoryVideos, VisualCategoryShortDramaFinal:
	default:
		return nil, errors.New("category 必须是 images、videos 或 short_drama_final")
	}
	if page < 1 {
		return nil, errors.New("page 必须大于等于 1")
	}
	if pageSize < 1 {
		return nil, errors.New("page_size 必须大于等于 1")
	}
	if c.paths == nil || strings.TrimSpace(c.paths.Visuals) == "" {
		return nil, errors.New("PAVO API 未配置 visuals 路径")
	}

	var response struct {
		Code    string      `json:"code"`
		Message string      `json:"message"`
		Data    VisualsData `json:"data"`
	}
	query := url.Values{
		"category":  {string(category)},
		"page":      {strconv.Itoa(page)},
		"page_size": {strconv.Itoa(pageSize)},
	}
	if err := c.getJSON(ctx, c.paths.Visuals, query, &response); err != nil {
		return nil, err
	}
	if err := validateEnvelope(response.Code, response.Message); err != nil {
		return nil, err
	}
	if response.Data.Groups == nil {
		response.Data.Groups = []VisualGroup{}
	}
	return &response.Data, nil
}

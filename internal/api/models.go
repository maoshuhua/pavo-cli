package api

import (
	"context"
	"errors"
	"net/url"
	"strings"
)

// ListModeSupportModels returns the server-configured model catalogue for one
// of the four top-level Pixa modes. Availability is intentionally queried at
// runtime because models and their online state change independently of CLI
// releases.
func (c *Client) ListModeSupportModels(ctx context.Context, mode ModeCode) ([]SupportedModel, error) {
	mode, err := normalizeModeCode(mode)
	if err != nil {
		return nil, err
	}
	if c.paths == nil || strings.TrimSpace(c.paths.ModeSupportModels) == "" {
		return nil, errors.New("PAVO API 未配置 mode support models 路径")
	}
	var response struct {
		Code    string `json:"code"`
		Message string `json:"message"`
		Data    struct {
			Models []SupportedModel `json:"models"`
		} `json:"data"`
	}
	if err := c.getJSON(ctx, c.paths.ModeSupportModels, url.Values{"mode_code": {string(mode)}}, &response); err != nil {
		return nil, err
	}
	if err := validateEnvelope(response.Code, response.Message); err != nil {
		return nil, err
	}
	if response.Data.Models == nil {
		response.Data.Models = []SupportedModel{}
	}
	return response.Data.Models, nil
}

func normalizeModeCode(mode ModeCode) (ModeCode, error) {
	normalized := ModeCode(strings.ToLower(strings.TrimSpace(string(mode))))
	switch normalized {
	case ModeCodeShortDrama, ModeCodeGenerateImage, ModeCodeGenerateVideo:
		return normalized, nil
	default:
		return "", errors.New("mode 必须是 short_drama、generate_image 或 generate_video")
	}
}

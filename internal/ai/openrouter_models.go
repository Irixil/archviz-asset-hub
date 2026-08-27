package ai

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"slices"
	"strconv"
)

type openRouterModel struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Description string `json:"description"`
	Pricing     struct {
		Image string `json:"image"`
	} `json:"pricing"`
	Architecture struct {
		Modality         string   `json:"modality"`
		InputModalities  []string `json:"input_modalities"`
		OutputModalities []string `json:"output_modalities"`
	} `json:"architecture"`
}

func fetchOpenRouterModels(ctx context.Context, apiKey, baseURL string, client *http.Client) ([]Model, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, baseURL+"/models", nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+apiKey)
	req.Header.Set("Http-Referer", openRouterReferer)
	req.Header.Set("X-Title", openRouterTitle)

	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode >= http.StatusBadRequest {
		return nil, fmt.Errorf("openrouter: list models: status %d", resp.StatusCode)
	}

	var envelope struct {
		Data []openRouterModel `json:"data"`
	}
	if err = json.Unmarshal(body, &envelope); err != nil {
		return nil, err
	}

	models := make([]Model, 0, len(envelope.Data))
	for _, m := range envelope.Data {
		price, _ := strconv.ParseFloat(m.Pricing.Image, 64)
		models = append(models, Model{
			ID:            m.ID,
			Name:          m.Name,
			PricePerImage: price,
			ProviderID:    ProviderOpenRouter,
			Capabilities:  modelCapability(m),
		})
	}
	return models, nil
}

func modelCapability(m openRouterModel) Capability {
	caps := Capability(0)
	if slices.Contains(m.Architecture.InputModalities, modImage) &&
		slices.Contains(m.Architecture.OutputModalities, modImage) {
		caps = caps | CapImageToImage | CapBgRemove
	}
	if slices.Contains(m.Architecture.InputModalities, modImage) &&
		slices.Contains(m.Architecture.OutputModalities, modText) {
		caps |= CapImageToText
	}
	return caps
}

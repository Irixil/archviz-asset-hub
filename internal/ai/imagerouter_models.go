package ai

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"sort"
	"strings"
	"time"
	"unicode"
)

type imageRouterModel struct {
	ID            string
	Name          string
	Provider      string
	PricePerImage float64
	Capabilities  Capability
}

type modelsResponseV2 []modelResponseV2

type modelResponseV2 struct {
	ID    string `json:"id"`
	Price struct {
		Min     float64 `json:"min"`
		Average float64 `json:"average"`
	} `json:"price"`
}

const fetchModelsTimeout = 15 * time.Second

func fetchImageRouterModels(ctx context.Context, apiKey, baseURL string) ([]imageRouterModel, error) {
	httpClient := &http.Client{Timeout: fetchModelsTimeout}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, imageRouterModelsEndpointURL(baseURL), nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+apiKey)

	resp, err := httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		return nil, errIRAPI
	}

	var parsed modelsResponseV2
	if err = json.Unmarshal(body, &parsed); err != nil {
		return nil, err
	}

	models := make([]imageRouterModel, 0, len(parsed))
	for _, raw := range parsed {
		price := raw.Price.Average
		if price == 0 {
			price = raw.Price.Min
		}
		caps := CapImageToImage
		if strings.Contains(raw.ID, "remove") {
			caps = CapBgRemove
		}
		models = append(models, normalizeImageRouterModel(imageRouterModel{
			ID:            raw.ID,
			PricePerImage: price,
			Capabilities:  caps,
		}))
	}
	return normalizeAndSortImageRouterModels(models), nil
}

func imageRouterModelsEndpointURL(baseURL string) string {
	base := strings.TrimSuffix(strings.TrimRight(baseURL, "/"), "/v1")
	return base + "/v2/models?inputType=image&outputType=image"
}

func normalizeAndSortImageRouterModels(in []imageRouterModel) []imageRouterModel {
	models := make([]imageRouterModel, 0, len(in))
	for _, model := range in {
		models = append(models, normalizeImageRouterModel(model))
	}
	sort.Slice(models, func(i, j int) bool {
		if models[i].PricePerImage == models[j].PricePerImage {
			return models[i].Name < models[j].Name
		}
		return models[i].PricePerImage < models[j].PricePerImage
	})
	return models
}

func normalizeImageRouterModel(model imageRouterModel) imageRouterModel {
	if provider, slug, ok := strings.Cut(model.ID, "/"); ok {
		if model.Provider == "" {
			model.Provider = provider
		}
		if model.Name == "" {
			model.Name = humanizeImageRouterModelSlug(slug)
		}
	}
	if model.Name == "" {
		model.Name = model.ID
	}
	if model.Provider == "" {
		model.Provider = model.ID
	}
	return model
}

func humanizeImageRouterModelSlug(slug string) string {
	slug = strings.NewReplacer("-", " ", "_", " ").Replace(slug)
	parts := strings.Fields(slug)
	for i, part := range parts {
		parts[i] = formatImageRouterModelToken(part)
	}
	return strings.Join(parts, " ")
}

func formatImageRouterModelToken(token string) string {
	if token == "" {
		return token
	}
	if isAllUpperToken(token) {
		return token
	}

	switch strings.ToLower(token) {
	case "bria":
		return "BRIA"
	case "ccsr":
		return "CCSR"
	case "gpt":
		return "GPT"
	case "qwen":
		return "Qwen"
	case "sd":
		return "SD"
	case "sd3":
		return "SD3"
	case "sdxl":
		return "SDXL"
	case "xai":
		return "xAI"
	case "xl":
		return "XL"
	}

	runes := []rune(strings.ToLower(token))
	runes[0] = unicode.ToUpper(runes[0])
	return string(runes)
}

func isAllUpperToken(token string) bool {
	hasLetter := false
	for _, r := range token {
		if unicode.IsLetter(r) {
			hasLetter = true
			if unicode.IsLower(r) {
				return false
			}
		}
	}
	return hasLetter
}

package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strings"
)

type AIResult struct {
	Provider  string   `json:"provider"`
	Model     string   `json:"model"`
	Text      string   `json:"text"`
	Citations []string `json:"citations,omitempty"`
}

type MarketauxResult struct {
	Endpoint   string            `json:"endpoint"`
	Parameters map[string]string `json:"parameters"`
	JSON       string            `json:"json"`
}

func (a *App) callOpenAIResponse(ctx context.Context, model, reasoningEffort, prompt string) (AIResult, error) {
	apiKey := strings.TrimSpace(os.Getenv("OPENAI_API_KEY"))
	if apiKey == "" {
		return AIResult{}, fmt.Errorf("OPENAI_API_KEY not set")
	}

	ctx, cancel := context.WithTimeout(ctx, a.Config.Providers.OpenAI.Timeout())
	defer cancel()

	payload := map[string]any{
		"model":        model,
		"instructions": "You are a concise news-summary assistant.",
		"input":        prompt,
	}
	if strings.TrimSpace(reasoningEffort) != "" {
		payload["reasoning"] = map[string]any{"effort": reasoningEffort}
	}

	body, err := a.postJSON(ctx, "https://api.openai.com/v1/responses", apiKey, payload, nil)
	if err != nil {
		return AIResult{}, err
	}

	text := extractOutputText(body)
	if text == "" {
		return AIResult{}, fmt.Errorf("openai returned no output text")
	}

	return AIResult{
		Provider: "openai",
		Model:    model,
		Text:     text,
	}, nil
}

type xAIRequest struct {
	Prompt           string
	Model            string
	UseWebSearch     bool
	UseXSearch       bool
	AllowedDomains   []string
	ExcludedDomains  []string
	AllowedXHandles  []string
	ExcludedXHandles []string
	FromDate         string
	ToDate           string
}

func (a *App) callXAIResponse(ctx context.Context, req xAIRequest) (AIResult, error) {
	apiKey := strings.TrimSpace(os.Getenv("GROK_API_KEY"))
	if apiKey == "" {
		return AIResult{}, fmt.Errorf("GROK_API_KEY not set")
	}

	ctx, cancel := context.WithTimeout(ctx, a.Config.Providers.Grok.Timeout())
	defer cancel()

	payload := map[string]any{
		"model": req.Model,
		"input": []map[string]any{
			{
				"role":    "system",
				"content": "You are a concise news and markets research assistant. Use any enabled search tools to ground your answer in current sources.",
			},
			{
				"role":    "user",
				"content": req.Prompt,
			},
		},
	}

	tools := buildXAITools(req)
	if len(tools) > 0 {
		payload["tools"] = tools
	}

	body, err := a.postJSON(ctx, "https://api.x.ai/v1/responses", apiKey, payload, nil)
	if err != nil {
		return AIResult{}, err
	}

	text := extractOutputText(body)
	if text == "" {
		return AIResult{}, fmt.Errorf("xAI returned no output text")
	}

	return AIResult{
		Provider:  "xai",
		Model:     req.Model,
		Text:      text,
		Citations: extractCitations(body),
	}, nil
}

func buildXAITools(req xAIRequest) []map[string]any {
	var tools []map[string]any

	if req.UseWebSearch {
		tool := map[string]any{"type": "web_search"}
		filters := map[string]any{}
		if len(req.AllowedDomains) > 0 {
			filters["allowed_domains"] = req.AllowedDomains
		}
		if len(req.ExcludedDomains) > 0 {
			filters["excluded_domains"] = req.ExcludedDomains
		}
		if len(filters) > 0 {
			tool["filters"] = filters
		}
		tools = append(tools, tool)
	}

	if req.UseXSearch {
		tool := map[string]any{"type": "x_search"}
		if len(req.AllowedXHandles) > 0 {
			tool["allowed_x_handles"] = req.AllowedXHandles
		}
		if len(req.ExcludedXHandles) > 0 {
			tool["excluded_x_handles"] = req.ExcludedXHandles
		}
		if strings.TrimSpace(req.FromDate) != "" {
			tool["from_date"] = strings.TrimSpace(req.FromDate)
		}
		if strings.TrimSpace(req.ToDate) != "" {
			tool["to_date"] = strings.TrimSpace(req.ToDate)
		}
		tools = append(tools, tool)
	}

	return tools
}

func (a *App) callPerplexityResponse(ctx context.Context, model, searchMode, query string) (AIResult, error) {
	apiKey := strings.TrimSpace(os.Getenv("PPLX_API_KEY"))
	if apiKey == "" {
		return AIResult{}, fmt.Errorf("PPLX_API_KEY not set")
	}

	ctx, cancel := context.WithTimeout(ctx, a.Config.Providers.Perplexity.Timeout())
	defer cancel()

	payload := map[string]any{
		"model": model,
		"messages": []map[string]string{
			{
				"role":    "system",
				"content": "You are a concise research assistant for traders.",
			},
			{
				"role":    "user",
				"content": query,
			},
		},
	}
	if strings.TrimSpace(searchMode) != "" {
		payload["search_mode"] = searchMode
	}

	body, err := a.postJSON(ctx, "https://api.perplexity.ai/chat/completions", apiKey, payload, map[string]string{
		"Accept": "application/json",
	})
	if err != nil {
		return AIResult{}, err
	}

	text := extractOutputText(body)
	if text == "" {
		return AIResult{}, fmt.Errorf("perplexity returned no output text")
	}

	return AIResult{
		Provider:  "perplexity",
		Model:     model,
		Text:      text,
		Citations: extractCitations(body),
	}, nil
}

func (a *App) callMarketaux(ctx context.Context, endpointPath string, params url.Values) (MarketauxResult, error) {
	apiKey := strings.TrimSpace(os.Getenv("MARKETAUX_API_KEY"))
	if apiKey == "" {
		return MarketauxResult{}, fmt.Errorf("MARKETAUX_API_KEY not set")
	}

	ctx, cancel := context.WithTimeout(ctx, a.Config.Providers.Marketaux.Timeout())
	defer cancel()

	params.Set("api_token", apiKey)
	endpointURL := "https://api.marketaux.com/v1/entity" + endpointPath + "?" + params.Encode()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpointURL, nil)
	if err != nil {
		return MarketauxResult{}, err
	}
	req.Header.Set("Accept", "application/json")

	resp, err := a.HTTPClient.Do(req)
	if err != nil {
		return MarketauxResult{}, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return MarketauxResult{}, err
	}
	if resp.StatusCode >= http.StatusBadRequest {
		return MarketauxResult{}, fmt.Errorf("marketaux error %d: %s", resp.StatusCode, strings.TrimSpace(string(body)))
	}

	out := map[string]string{}
	for key, values := range params {
		if key == "api_token" || len(values) == 0 {
			continue
		}
		out[key] = values[0]
	}

	return MarketauxResult{
		Endpoint:   endpointPath,
		Parameters: out,
		JSON:       string(body),
	}, nil
}

func (a *App) postJSON(ctx context.Context, endpoint, apiKey string, payload any, headers map[string]string) ([]byte, error) {
	body, err := json.Marshal(payload)
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+apiKey)
	for key, value := range headers {
		req.Header.Set(key, value)
	}

	resp, err := a.HTTPClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode >= http.StatusBadRequest {
		return nil, fmt.Errorf("%s returned %d: %s", endpoint, resp.StatusCode, strings.TrimSpace(string(respBody)))
	}

	return respBody, nil
}

func extractOutputText(body []byte) string {
	var envelope struct {
		OutputText string `json:"output_text"`
		Output     []struct {
			Type    string `json:"type"`
			Role    string `json:"role"`
			Content []struct {
				Type    string `json:"type"`
				Text    string `json:"text"`
				Refusal string `json:"refusal"`
			} `json:"content"`
		} `json:"output"`
		Choices []struct {
			Message struct {
				Content string `json:"content"`
			} `json:"message"`
		} `json:"choices"`
	}
	if err := json.Unmarshal(body, &envelope); err != nil {
		return ""
	}

	if strings.TrimSpace(envelope.OutputText) != "" {
		return strings.TrimSpace(envelope.OutputText)
	}

	var parts []string
	for _, output := range envelope.Output {
		for _, content := range output.Content {
			switch content.Type {
			case "output_text", "text":
				if text := strings.TrimSpace(content.Text); text != "" {
					parts = append(parts, text)
				}
			case "refusal":
				if text := strings.TrimSpace(content.Refusal); text != "" {
					parts = append(parts, text)
				}
			}
		}
	}
	if len(parts) > 0 {
		return strings.Join(parts, "\n\n")
	}

	if len(envelope.Choices) > 0 {
		return strings.TrimSpace(envelope.Choices[0].Message.Content)
	}

	return ""
}

func extractCitations(body []byte) []string {
	var raw map[string]any
	if err := json.Unmarshal(body, &raw); err != nil {
		return nil
	}

	seen := map[string]struct{}{}
	var out []string

	appendCitation := func(s string) {
		s = strings.TrimSpace(s)
		if s == "" {
			return
		}
		if _, ok := seen[s]; ok {
			return
		}
		seen[s] = struct{}{}
		out = append(out, s)
	}

	if citations, ok := raw["citations"].([]any); ok {
		for _, item := range citations {
			switch v := item.(type) {
			case string:
				appendCitation(v)
			case map[string]any:
				appendCitation(formatCitationMap(v))
			}
		}
	}

	if results, ok := raw["search_results"].([]any); ok {
		for _, item := range results {
			if v, ok := item.(map[string]any); ok {
				appendCitation(formatCitationMap(v))
			}
		}
	}

	return out
}

func formatCitationMap(m map[string]any) string {
	title := firstString(m["title"], m["name"])
	link := firstString(m["url"], m["link"])
	if title != "" && link != "" {
		return title + " - " + link
	}
	if link != "" {
		return link
	}
	return title
}

func firstString(values ...any) string {
	for _, value := range values {
		if s, ok := value.(string); ok && strings.TrimSpace(s) != "" {
			return strings.TrimSpace(s)
		}
	}
	return ""
}

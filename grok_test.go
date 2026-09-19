package main

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

type grokTransport func(*http.Request) (*http.Response, error)

func (f grokTransport) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

func TestGrokRequestAndUsage(t *testing.T) {
	t.Setenv("GROK_API_KEY", "test-key")
	for _, tc := range []struct {
		name, effort        string
		key                 *string
		wantEffort, wantKey string
	}{
		{name: "defaults", wantEffort: "high", wantKey: "news-routine-mcp"},
		{name: "override", effort: "xhigh", key: stringPtr("research"), wantEffort: "xhigh", wantKey: "research"},
		{name: "omit routing", effort: "low", key: stringPtr(""), wantEffort: "low"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			app := &App{Config: defaultConfig()}
			app.HTTPClient = &http.Client{Transport: grokTransport(func(r *http.Request) (*http.Response, error) {
				if r.URL.String() != "https://api.x.ai/v1/responses" {
					t.Errorf("unexpected endpoint %s", r.URL)
				}
				var payload map[string]any
				if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
					t.Fatal(err)
				}
				if payload["model"] != "grok-4.6" {
					t.Errorf("model: %v", payload["model"])
				}
				effort := payload["reasoning"].(map[string]any)["effort"]
				if effort != tc.wantEffort {
					t.Errorf("effort: %v", effort)
				}
				key, present := payload["prompt_cache_key"]
				if tc.wantKey == "" {
					if present {
						t.Error("expected no cache key")
					}
				} else if key != tc.wantKey {
					t.Errorf("cache key: %v", key)
				}
				input := payload["input"].([]any)
				if input[0].(map[string]any)["role"] != "system" || input[1].(map[string]any)["content"] != "Latest news?" {
					t.Errorf("unexpected input: %v", input)
				}
				if len(payload["tools"].([]any)) != 2 {
					t.Error("expected both search tools")
				}
				return &http.Response{StatusCode: 200, Body: io.NopCloser(strings.NewReader(`{"output_text":"Fresh news","usage":{"input_tokens":100,"input_tokens_details":{"cached_tokens":80},"output_tokens":40,"output_tokens_details":{"reasoning_tokens":25},"total_tokens":140}}`)), Header: make(http.Header)}, nil
			})}
			_, result, err := app.handleRunGrokPrompt(context.Background(), nil, GrokPromptInput{Prompt: "Latest news?", ReasoningEffort: tc.effort, PromptCacheKey: tc.key})
			if err != nil {
				t.Fatal(err)
			}
			if result.Text != "Fresh news" || result.Usage == nil {
				t.Fatalf("unexpected result: %+v", result)
			}
			u := result.Usage
			if u.InputTokens != 100 || u.InputTokensDetails.CachedTokens != 80 || u.OutputTokens != 40 || u.OutputTokensDetails.ReasoningTokens != 25 || u.TotalTokens != 140 {
				t.Fatalf("usage: %+v", u)
			}
			if !strings.Contains(renderAIResult(result), "100 input (80 cached), 40 output (25 reasoning), 140 total") {
				t.Error("missing usage footer")
			}
		})
	}
}

func TestGrokConfig(t *testing.T) {
	for _, tc := range []struct {
		name, yaml, effort, key string
		invalid                 bool
	}{
		{name: "omitted", yaml: "{}", effort: "high", key: "news-routine-mcp"},
		{name: "overrides", yaml: "providers:\n  grok:\n    reasoning_effort: medium\n    prompt_cache_key: research\n", effort: "medium", key: "research"},
		{name: "empty", yaml: "providers:\n  grok:\n    reasoning_effort: ''\n    prompt_cache_key: ''\n", effort: "high"},
		{name: "invalid", yaml: "providers:\n  grok:\n    reasoning_effort: max\n", invalid: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "config.yaml")
			if err := os.WriteFile(path, []byte(tc.yaml), 0600); err != nil {
				t.Fatal(err)
			}
			cfg, err := loadConfig(path)
			if tc.invalid {
				if err == nil {
					t.Fatal("expected error")
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			if cfg.Providers.Grok.ReasoningEffort != tc.effort || cfg.Providers.Grok.PromptCacheKey != tc.key {
				t.Fatalf("config: %+v", cfg.Providers.Grok)
			}
		})
	}
}

func TestGrokInvalidEffortBeforeNetwork(t *testing.T) {
	app := &App{Config: defaultConfig(), HTTPClient: &http.Client{Transport: grokTransport(func(*http.Request) (*http.Response, error) { t.Fatal("unexpected network request"); return nil, nil })}}
	_, _, err := app.handleRunGrokPrompt(context.Background(), nil, GrokPromptInput{Prompt: "News?", ReasoningEffort: "invalid"})
	if err == nil || !strings.Contains(err.Error(), "reasoning_effort") {
		t.Fatalf("expected validation error, got %v", err)
	}
}

func TestGrokUsageAbsentAndZero(t *testing.T) {
	if extractAIUsage([]byte(`{"output_text":"News"}`)) != nil {
		t.Fatal("missing usage should stay absent")
	}
	u := extractAIUsage([]byte(`{"usage":{"input_tokens":10,"input_tokens_details":{"cached_tokens":0}}}`))
	if u == nil || u.InputTokensDetails.CachedTokens != 0 {
		t.Fatal("zero cached tokens must be preserved")
	}
	if renderAIResult(AIResult{Text: "News"}) != "News" {
		t.Fatal("unexpected footer for missing usage")
	}
	// Constructing the MCP server also validates inferred input/output schemas.
	newMCPServer(&App{Config: defaultConfig()})
}

func stringPtr(s string) *string { return &s }

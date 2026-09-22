package main

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"image"
	"image/color"
	"image/gif"
	"io"
	"net/http"
	"strings"
	"testing"
)

func TestPerplexityImageRequest(t *testing.T) {
	t.Setenv("PPLX_API_KEY", "test-key")
	for _, tc := range []struct {
		name, model, searchMode string
		images                  []string
	}{
		{name: "text"},
		{name: "default model with mixed attachments", images: []string{testPNG, "https://example.com/chart.webp"}},
		{name: "sonar", model: "sonar", searchMode: "web", images: []string{testPNG}},
		{name: "reasoning", model: "sonar-reasoning-pro", images: []string{testPNG}},
		{name: "deep research text still allowed", model: "sonar-deep-research"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			called := false
			app := &App{Config: defaultConfig()}
			app.HTTPClient = &http.Client{Transport: grokTransport(func(r *http.Request) (*http.Response, error) {
				called = true
				if r.URL.String() != "https://api.perplexity.ai/chat/completions" || r.Header.Get("Authorization") != "Bearer test-key" {
					t.Fatal("incorrect endpoint/auth")
				}
				var p map[string]any
				if err := json.NewDecoder(r.Body).Decode(&p); err != nil {
					t.Fatal(err)
				}
				if p["model"] != fallbackString(tc.model, "sonar-pro") || p["search_mode"] != fallbackString(tc.searchMode, "sec") {
					t.Fatalf("model/search settings lost: %v", p)
				}
				messages := p["messages"].([]any)
				if len(messages) != 2 || messages[0].(map[string]any)["role"] != "system" {
					t.Fatal("system prompt lost")
				}
				user := messages[1].(map[string]any)
				if user["role"] != "user" {
					t.Fatal("wrong role")
				}
				if len(tc.images) == 0 {
					if user["content"] != "Explain" {
						t.Fatal("text request changed")
					}
				} else {
					parts := user["content"].([]any)
					if len(parts) != len(tc.images)+1 || parts[0].(map[string]any)["type"] != "text" || parts[0].(map[string]any)["text"] != "Explain" {
						t.Fatal("incorrect text part")
					}
					for i, image := range tc.images {
						part := parts[i+1].(map[string]any)
						if part["type"] != "image_url" || part["image_url"].(map[string]any)["url"] != image {
							t.Fatalf("incorrect image part %d", i)
						}
					}
				}
				return &http.Response{StatusCode: 200, Body: io.NopCloser(strings.NewReader(`{"choices":[{"message":{"content":"Explanation"}}],"citations":["https://example.com/source"]}`))}, nil
			})}
			_, result, err := app.handleRunPerplexityQuery(context.Background(), nil, PerplexityQueryInput{Query: " Explain ", Model: tc.model, SearchMode: tc.searchMode, ImageURLs: tc.images})
			if err != nil || !called || result.Text != "Explanation" || result.Provider != "perplexity" || len(result.Citations) != 1 {
				t.Fatalf("called=%v result=%+v err=%v", called, result, err)
			}
		})
	}
}

func TestPerplexityInvalidImagesBeforeNetwork(t *testing.T) {
	app := &App{Config: defaultConfig(), HTTPClient: &http.Client{Transport: grokTransport(func(*http.Request) (*http.Response, error) { t.Fatal("unexpected network request"); return nil, nil })}}
	for _, image := range []string{
		"", "/tmp/chart.png", "file:///tmp/chart.png", "http://example.com/image.png", "https://", "https://user:pass@example.com/image.png",
		"data:audio/wav;base64,AAAA", "data:image/svg+xml;base64,AAAA", "data:image/png;base64,", "data:image/png;base64,!!!!",
		"data:image/png;base64,AAAA", strings.Replace(testPNG, "image/png", "image/jpeg", 1),
	} {
		_, _, err := app.handleRunPerplexityQuery(context.Background(), nil, PerplexityQueryInput{Query: "Explain", ImageURLs: []string{image}})
		if err == nil || !strings.Contains(err.Error(), "image_urls[0]") {
			t.Fatalf("expected image validation error, got %v", err)
		}
	}
	for _, model := range []string{"sonar-deep-research", "unknown"} {
		_, _, err := app.handleRunPerplexityQuery(context.Background(), nil, PerplexityQueryInput{Query: "Explain", Model: model, ImageURLs: []string{testPNG}})
		if err == nil || !strings.Contains(err.Error(), "not enabled") {
			t.Fatalf("expected capability error, got %v", err)
		}
	}
	_, _, err := app.handleRunPerplexityQuery(context.Background(), nil, PerplexityQueryInput{ImageURLs: []string{testPNG}})
	if err == nil || err.Error() != "query is required" {
		t.Fatal("expected required query")
	}
	tooLarge := "data:image/png;base64," + strings.Repeat("A", base64.StdEncoding.EncodedLen(maxPerplexityImageBytes)+4)
	if err := validatePerplexityImage(tooLarge); err == nil || !strings.Contains(err.Error(), "50 MB") {
		t.Fatal("expected size error")
	}
}

func TestPerplexityImageFormats(t *testing.T) {
	frame := image.NewPaletted(image.Rect(0, 0, 1, 1), color.Palette{color.Black})
	var gifBytes bytes.Buffer
	if err := gif.Encode(&gifBytes, frame, nil); err != nil {
		t.Fatal(err)
	}
	for _, image := range []string{
		testPNG,
		"data:image/jpeg;base64," + base64.StdEncoding.EncodeToString([]byte{0xff, 0xd8, 0xff, 0xe0}),
		"data:image/gif;base64," + base64.StdEncoding.EncodeToString(gifBytes.Bytes()),
		"data:image/webp;base64,UklGRiIAAABXRUJQVlA4IBYAAAAwAQCdASoBAAEADsD+JaQAA3AAAAAA",
	} {
		if err := validatePerplexityImage(image); err != nil {
			t.Fatal(err)
		}
	}
}

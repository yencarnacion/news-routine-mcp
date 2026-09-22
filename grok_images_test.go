package main

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"testing"
)

const testPNG = "data:image/png;base64,iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAQAAAC1HAwCAAAAC0lEQVR42mP8/x8AAwMCAO+jRZkAAAAASUVORK5CYII="

func TestGrokImagesRequest(t *testing.T) {
	t.Setenv("GROK_API_KEY", "test-key")
	images := []string{"https://example.com/chart.png", testPNG}
	called := false
	app := &App{Config: defaultConfig(), HTTPClient: &http.Client{Transport: grokTransport(func(r *http.Request) (*http.Response, error) {
		called = true
		var payload struct {
			Model string `json:"model"`
			Store *bool  `json:"store"`
			Input []struct {
				Content json.RawMessage `json:"content"`
			} `json:"input"`
		}
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			t.Fatal(err)
		}
		if payload.Model != "grok-4.7" || payload.Store == nil || *payload.Store {
			t.Fatalf("incorrect model/storage: %+v", payload)
		}
		var parts []map[string]string
		if err := json.Unmarshal(payload.Input[1].Content, &parts); err != nil {
			t.Fatal(err)
		}
		if len(parts) != 3 || parts[0]["type"] != "input_text" || parts[0]["text"] != "Explain" {
			t.Fatalf("unexpected content: %v", parts)
		}
		for i, image := range images {
			if parts[i+1]["type"] != "input_image" || parts[i+1]["image_url"] != image {
				t.Fatalf("incorrect image %d", i)
			}
		}
		return &http.Response{StatusCode: 200, Body: io.NopCloser(strings.NewReader(`{"output_text":"Chart explanation"}`))}, nil
	})}}
	_, result, err := app.handleRunGrokPrompt(context.Background(), nil, GrokPromptInput{Prompt: "Explain", ImageURLs: images})
	if err != nil || !called || result.Text != "Chart explanation" {
		t.Fatalf("called=%v result=%+v err=%v", called, result, err)
	}
}

func TestGrokInvalidImagesBeforeNetwork(t *testing.T) {
	app := &App{Config: defaultConfig(), HTTPClient: &http.Client{Transport: grokTransport(func(*http.Request) (*http.Response, error) { t.Fatal("unexpected network request"); return nil, nil })}}
	for _, image := range []string{
		"", "/tmp/chart.png", "file:///tmp/chart.png", "https://", "https://user:pass@example.com/a.png",
		"data:audio/wav;base64,AAAA", "data:image/gif;base64,AAAA", "data:image/png;base64,", "data:image/png;base64,!!!!",
		"data:image/png;base64," + base64.StdEncoding.EncodeToString([]byte("not an image")),
		strings.Replace(testPNG, "image/png", "image/jpeg", 1),
		"data:image/png;base64," + strings.Repeat("A", base64.StdEncoding.EncodedLen(maxGrokImageBytes)+4),
	} {
		_, _, err := app.handleRunGrokPrompt(context.Background(), nil, GrokPromptInput{Prompt: "Explain", ImageURLs: []string{image}})
		if err == nil || !strings.Contains(err.Error(), "image_urls[0]") {
			t.Fatalf("expected indexed image validation error, got %v", err)
		}
	}
}

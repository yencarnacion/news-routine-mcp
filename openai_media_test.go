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

func TestOpenAIImageFormats(t *testing.T) {
	frame := image.NewPaletted(image.Rect(0, 0, 1, 1), color.Palette{color.Black})
	for _, frames := range []int{1, 2} {
		var encoded bytes.Buffer
		g := &gif.GIF{}
		for i := 0; i < frames; i++ {
			g.Image = append(g.Image, frame)
			g.Delay = append(g.Delay, 1)
		}
		if err := gif.EncodeAll(&encoded, g); err != nil {
			t.Fatal(err)
		}
		err := validateOpenAIImage("data:image/gif;base64," + base64.StdEncoding.EncodeToString(encoded.Bytes()))
		if (err == nil) != (frames == 1) {
			t.Fatalf("frames=%d err=%v", frames, err)
		}
	}
	// A 1x1 WebP fixture checks WebP MIME detection and pass-through.
	webp := "data:image/webp;base64,UklGRiIAAABXRUJQVlA4IBYAAAAwAQCdASoBAAEADsD+JaQAA3AAAAAA"
	if err := validateOpenAIImage(webp); err != nil {
		t.Fatal(err)
	}
	if err := validateOpenAIImage(strings.Replace(webp, "image/webp", "image/png", 1)); err == nil {
		t.Fatal("expected MIME mismatch")
	}
}

func TestOpenAIMediaRequests(t *testing.T) {
	t.Setenv("OPENAI_API_KEY", "test-key")
	wav := OpenAIAudioInput{Data: base64.StdEncoding.EncodeToString([]byte("RIFF\x24\x00\x00\x00WAVEfmt ")), Format: "wav"}
	for _, tc := range []struct {
		name string
		in   SummarizeTradeTheNewsInput
		chat bool
	}{
		{"text", SummarizeTradeTheNewsInput{Email: "Market news"}, false},
		{"images only", SummarizeTradeTheNewsInput{ImageURLs: []string{testPNG, "https://example.com/chart.webp"}}, false},
		{"image and text", SummarizeTradeTheNewsInput{Email: "Market news", ImageURLs: []string{testPNG}}, false},
		{"audio only", SummarizeTradeTheNewsInput{Model: "gpt-audio-1.5", Audio: []OpenAIAudioInput{wav}}, true},
		{"audio model text", SummarizeTradeTheNewsInput{Model: "gpt-audio-mini", Email: "Market news"}, true},
		{"older vision model", SummarizeTradeTheNewsInput{Model: "gpt-4.1", ImageURLs: []string{testPNG}}, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			called := false
			app := &App{Config: defaultConfig(), Settings: Settings{NewsPrompt: "Summarize the source"}}
			app.HTTPClient = &http.Client{Transport: grokTransport(func(r *http.Request) (*http.Response, error) {
				called = true
				if r.Header.Get("Authorization") != "Bearer test-key" {
					t.Error("missing auth")
				}
				var p map[string]any
				if err := json.NewDecoder(r.Body).Decode(&p); err != nil {
					t.Fatal(err)
				}
				if p["model"] != fallbackString(tc.in.Model, "gpt-5.6-terra") {
					t.Fatalf("model: %v", p["model"])
				}
				var content []any
				if tc.chat {
					if r.URL.Path != "/v1/chat/completions" || p["reasoning"] != nil || p["reasoning_effort"] != nil || p["store"] != false {
						t.Fatalf("incorrect audio request: %v", p)
					}
					if p["modalities"].([]any)[0] != "text" {
						t.Error("expected text output")
					}
					messages := p["messages"].([]any)
					content = messages[1].(map[string]any)["content"].([]any)
					if len(content) != len(tc.in.Audio)+1 {
						t.Fatal("missing audio")
					}
					for i, clip := range tc.in.Audio {
						part := content[i+1].(map[string]any)
						a := part["input_audio"].(map[string]any)
						if part["type"] != "input_audio" || a["data"] != clip.Data || a["format"] != clip.Format {
							t.Fatal("wrong audio content")
						}
					}
				} else {
					if r.URL.Path != "/v1/responses" {
						t.Fatal("wrong endpoint")
					}
					if tc.in.Model == "gpt-4.1" && p["reasoning"] != nil {
						t.Fatal("unsupported reasoning")
					}
					if len(tc.in.ImageURLs) > 0 {
						if p["store"] != false {
							t.Fatal("expected store false")
						}
						content = p["input"].([]any)[0].(map[string]any)["content"].([]any)
						if len(content) != len(tc.in.ImageURLs)+1 {
							t.Fatal("missing images")
						}
						for i, image := range tc.in.ImageURLs {
							part := content[i+1].(map[string]any)
							if part["type"] != "input_image" || part["image_url"] != image {
								t.Fatal("wrong image content")
							}
						}
					} else if p["input"] != "Summarize the source\n\nMarket news" {
						t.Fatal("text behavior changed")
					}
				}
				if len(content) > 0 && !strings.HasPrefix(content[0].(map[string]any)["text"].(string), "Summarize the source") {
					t.Fatal("lost preset")
				}
				body := `{"output_text":"Summary"}`
				if tc.chat {
					body = `{"choices":[{"message":{"content":"Summary"}}]}`
				}
				return &http.Response{StatusCode: 200, Body: io.NopCloser(strings.NewReader(body))}, nil
			})}
			_, result, err := app.handleSummarizeTradeTheNews(context.Background(), nil, tc.in)
			if err != nil || !called || result.Text != "Summary" {
				t.Fatalf("called=%v result=%+v err=%v", called, result, err)
			}
		})
	}
}

func TestOpenAIRejectsInvalidMedia(t *testing.T) {
	app := &App{Config: defaultConfig(), HTTPClient: &http.Client{Transport: grokTransport(func(*http.Request) (*http.Response, error) { t.Fatal("unexpected network request"); return nil, nil })}}
	for _, in := range []SummarizeTradeTheNewsInput{
		{},
		{Audio: []OpenAIAudioInput{{Data: "AAAA", Format: "wav"}}},
		{Model: "gpt-audio-1.5", ImageURLs: []string{testPNG}},
		{Model: "gpt-audio-1.5", Audio: []OpenAIAudioInput{{Data: "AAAA", Format: "ogg"}}},
		{Model: "gpt-audio-1.5", Audio: []OpenAIAudioInput{{Data: "!!!!", Format: "wav"}}},
		{Model: "gpt-audio-1.5", Audio: []OpenAIAudioInput{{Data: "AAAA", Format: "wav"}}},
		{ImageURLs: []string{"file:///tmp/image.png"}},
		{ImageURLs: []string{"data:image/png;base64,!!!!"}},
		{ImageURLs: []string{"data:image/svg+xml;base64,AAAA"}},
	} {
		if _, _, err := app.handleSummarizeTradeTheNews(context.Background(), nil, in); err == nil {
			t.Fatal("expected validation error")
		}
	}
	if _, err := decodeOpenAIUpload(strings.Repeat("A", base64.StdEncoding.EncodedLen(20*1024*1024)+4), 20); err == nil {
		t.Fatal("expected size error")
	}
	if err := validateOpenAIAudio(OpenAIAudioInput{Data: base64.StdEncoding.EncodeToString([]byte("ID3test")), Format: "mp3"}); err != nil {
		t.Fatal(err)
	}
}

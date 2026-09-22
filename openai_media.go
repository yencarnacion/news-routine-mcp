package main

import (
	"bytes"
	"encoding/base64"
	"fmt"
	"image/gif"
	"net/http"
	"strings"
)

type OpenAIAudioInput struct {
	Data   string `json:"data" jsonschema:"Base64-encoded audio file bytes (not a URL or local path)."`
	Format string `json:"format" jsonschema:"Audio file format: wav or mp3."`
}

// Keep this list explicit: audio models use a different endpoint and do not
// support image input or the default reasoning effort. Unknown models are not
// silently substituted or treated as audio-capable based on a name prefix.
func isOpenAIAudioModel(model string) bool {
	switch model {
	case "gpt-audio-1.5", "gpt-audio", "gpt-audio-2025-08-28",
		"gpt-audio-mini", "gpt-audio-mini-2025-10-06", "gpt-audio-mini-2025-12-15":
		return true
	}
	return false
}

func buildOpenAIRequest(model, effort, prompt string, images []string, audio []OpenAIAudioInput) (string, map[string]any, error) {
	const instructions = "You are a concise news-summary assistant."
	isAudio := isOpenAIAudioModel(model)
	if len(audio) > 0 && !isAudio {
		return "", nil, fmt.Errorf("audio uploads require a supported audio model, such as gpt-audio-1.5; selected model %q is not supported for audio", model)
	}
	if isAudio && len(images) > 0 {
		return "", nil, fmt.Errorf("model %q does not support image input; use a vision model for images or send audio separately", model)
	}
	if isAudio {
		content := []map[string]any{{"type": "text", "text": prompt}}
		for i, clip := range audio {
			if err := validateOpenAIAudio(clip); err != nil {
				return "", nil, fmt.Errorf("audio[%d]: %w", i, err)
			}
			content = append(content, map[string]any{"type": "input_audio", "input_audio": clip})
		}
		return "https://api.openai.com/v1/chat/completions", map[string]any{
			"model": model, "modalities": []string{"text"}, "store": false,
			"messages": []map[string]any{
				{"role": "system", "content": instructions},
				{"role": "user", "content": content},
			},
		}, nil
	}
	var input any = prompt
	if len(images) > 0 {
		content := []map[string]any{{"type": "input_text", "text": prompt}}
		for i, image := range images {
			image = strings.TrimSpace(image)
			if err := validateOpenAIImage(image); err != nil {
				return "", nil, fmt.Errorf("image_urls[%d]: %w", i, err)
			}
			content = append(content, map[string]any{"type": "input_image", "image_url": image})
		}
		input = []map[string]any{{"role": "user", "content": content}}
	}
	payload := map[string]any{"model": model, "instructions": instructions, "input": input}
	if len(images) > 0 {
		payload["store"] = false
	}
	if strings.TrimSpace(effort) != "" && !strings.HasPrefix(model, "gpt-4.1") && !strings.HasPrefix(model, "gpt-4o") {
		payload["reasoning"] = map[string]any{"effort": effort}
	}
	return "https://api.openai.com/v1/responses", payload, nil
}

func decodeOpenAIUpload(encoded string, maxMiB int) ([]byte, error) {
	limit := maxMiB * 1024 * 1024
	if len(encoded) > base64.StdEncoding.EncodedLen(limit) {
		return nil, fmt.Errorf("upload exceeds server limit of %d MiB", maxMiB)
	}
	data, err := base64.StdEncoding.Strict().DecodeString(encoded)
	if err != nil || len(data) == 0 {
		return nil, fmt.Errorf("upload contains empty or invalid base64 data")
	}
	if len(data) > limit {
		return nil, fmt.Errorf("upload exceeds server limit of %d MiB", maxMiB)
	}
	return data, nil
}

func validateOpenAIImage(image string) error {
	if !strings.HasPrefix(image, "data:") {
		return validateGrokImage(image) // Same HTTP(S) URL rules; no local fetching.
	}
	header, encoded, ok := strings.Cut(image, ",")
	if !ok {
		return fmt.Errorf("expected a base64 image data URL")
	}
	mime := strings.TrimSuffix(strings.TrimPrefix(header, "data:"), ";base64")
	if !strings.HasSuffix(header, ";base64") || (mime != "image/png" && mime != "image/jpeg" && mime != "image/webp" && mime != "image/gif") {
		return fmt.Errorf("image uploads must be PNG, JPEG, WebP, or non-animated GIF base64 data URLs")
	}
	data, err := decodeOpenAIUpload(encoded, 20)
	if err != nil {
		return err
	}
	if http.DetectContentType(data) != mime {
		return fmt.Errorf("image data does not match its media type")
	}
	if mime == "image/gif" {
		// Reject animated GIF uploads before sending them to the provider.
		g, err := gif.DecodeAll(bytes.NewReader(data))
		if err != nil || len(g.Image) != 1 {
			return fmt.Errorf("GIF must be valid and non-animated")
		}
	}
	return nil
}

func validateOpenAIAudio(clip OpenAIAudioInput) error {
	if clip.Format != "wav" && clip.Format != "mp3" {
		return fmt.Errorf("format must be wav or mp3")
	}
	data, err := decodeOpenAIUpload(clip.Data, 25)
	if err != nil {
		return err
	}
	wav := len(data) >= 12 && string(data[:4]) == "RIFF" && string(data[8:12]) == "WAVE"
	mp3 := bytes.HasPrefix(data, []byte("ID3")) || (len(data) >= 2 && data[0] == 0xff && data[1]&0xe0 == 0xe0)
	if (clip.Format == "wav" && !wav) || (clip.Format == "mp3" && !mp3) {
		return fmt.Errorf("audio data does not match its %s format", clip.Format)
	}
	return nil
}

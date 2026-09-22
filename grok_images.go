package main

import (
	"encoding/base64"
	"fmt"
	"net/http"
	"net/url"
	"strings"
)

const maxGrokImageBytes = 20 * 1024 * 1024

func grokUserContent(prompt string, images []string) (any, error) {
	if len(images) == 0 {
		return prompt, nil
	}
	content := []map[string]any{{"type": "input_text", "text": prompt}}
	for i, image := range images {
		image = strings.TrimSpace(image)
		if err := validateGrokImage(image); err != nil {
			return nil, fmt.Errorf("image_urls[%d]: %w", i, err)
		}
		content = append(content, map[string]any{"type": "input_image", "image_url": image})
	}
	return content, nil
}

func validateGrokImage(image string) error {
	if strings.HasPrefix(image, "data:") {
		header, encoded, ok := strings.Cut(image, ",")
		if !ok || (header != "data:image/jpeg;base64" && header != "data:image/png;base64") {
			return fmt.Errorf("uploads must be base64 JPEG or PNG data URLs; audio is not supported")
		}
		if len(encoded) > base64.StdEncoding.EncodedLen(maxGrokImageBytes) {
			return fmt.Errorf("image exceeds 20 MiB")
		}
		data, err := base64.StdEncoding.Strict().DecodeString(encoded)
		if err != nil || len(data) == 0 {
			return fmt.Errorf("image contains empty or invalid base64 data")
		}
		if len(data) > maxGrokImageBytes {
			return fmt.Errorf("image exceeds 20 MiB")
		}
		mime := strings.TrimSuffix(strings.TrimPrefix(header, "data:"), ";base64")
		if http.DetectContentType(data) != mime {
			return fmt.Errorf("image data must match its JPEG or PNG media type")
		}
		return nil
	}
	// xAI fetches remote images; no server-side URL fetch or local file access.
	u, err := url.Parse(image)
	if err != nil || (u.Scheme != "https" && u.Scheme != "http") || u.Hostname() == "" || u.User != nil {
		return fmt.Errorf("expected a public HTTP(S) image URL or base64 JPEG/PNG data URL")
	}
	return nil
}

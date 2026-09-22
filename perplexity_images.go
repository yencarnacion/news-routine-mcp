package main

import (
	"encoding/base64"
	"fmt"
	"net/http"
	"net/url"
	"strings"
)

// Perplexity documents its upload limit in MB, rather than MiB.
const maxPerplexityImageBytes = 50 * 1000 * 1000

func perplexityUserContent(model, query string, images []string) (any, error) {
	if len(images) == 0 {
		return query, nil
	}
	switch model {
	case "sonar", "sonar-pro", "sonar-reasoning-pro":
	default:
		return nil, fmt.Errorf("image attachments are not enabled for Perplexity model %q; use sonar, sonar-pro, or sonar-reasoning-pro", model)
	}
	content := []map[string]any{{"type": "text", "text": query}}
	for i, image := range images {
		image = strings.TrimSpace(image)
		if err := validatePerplexityImage(image); err != nil {
			return nil, fmt.Errorf("image_urls[%d]: %w", i, err)
		}
		content = append(content, map[string]any{
			"type": "image_url", "image_url": map[string]string{"url": image},
		})
	}
	return content, nil
}

func validatePerplexityImage(image string) error {
	if !strings.HasPrefix(image, "data:") {
		u, err := url.Parse(image)
		if err != nil || u.Scheme != "https" || u.Hostname() == "" || u.User != nil {
			return fmt.Errorf("expected a public HTTPS image URL or base64 image data URL")
		}
		// Perplexity fetches the image; the MCP server does not read local files
		// or fetch arbitrary URLs to validate their contents.
		return nil
	}
	header, encoded, ok := strings.Cut(image, ",")
	mime := strings.TrimSuffix(strings.TrimPrefix(header, "data:"), ";base64")
	if !ok || !strings.HasSuffix(header, ";base64") || (mime != "image/png" && mime != "image/jpeg" && mime != "image/webp" && mime != "image/gif") {
		return fmt.Errorf("uploads must be base64 PNG, JPEG, WebP, or GIF data URLs; Sonar audio input is not supported")
	}
	if len(encoded) > base64.StdEncoding.EncodedLen(maxPerplexityImageBytes) {
		return fmt.Errorf("image exceeds 50 MB")
	}
	data, err := base64.StdEncoding.Strict().DecodeString(encoded)
	if err != nil || len(data) == 0 {
		return fmt.Errorf("image contains empty or invalid base64 data")
	}
	if len(data) > maxPerplexityImageBytes {
		return fmt.Errorf("image exceeds 50 MB")
	}
	if http.DetectContentType(data) != mime {
		return fmt.Errorf("image data does not match its media type")
	}
	return nil
}

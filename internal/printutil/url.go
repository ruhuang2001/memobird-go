package printutil

import (
	"fmt"
	"net/url"
)

// ValidateURL requires an HTTP(S) URL with a host.
func ValidateURL(pageURL string) error {
	parsedURL, err := url.Parse(pageURL)
	if err != nil {
		return fmt.Errorf("invalid URL format: %w", err)
	}
	if parsedURL.Scheme != "http" && parsedURL.Scheme != "https" {
		return fmt.Errorf("unsupported URL scheme: %s (only http/https allowed)", parsedURL.Scheme)
	}
	if parsedURL.Hostname() == "" {
		return fmt.Errorf("URL host is required")
	}
	return nil
}

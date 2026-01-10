package formatter

import (
	"encoding/base64"
	"fmt"
	"strings"

	"golang.org/x/text/encoding/simplifiedchinese"
	"golang.org/x/text/transform"
)

func EncodeTextToGBKBase64(text string) (string, error) {
	if !strings.HasSuffix(text, "\n") {
		text = text + "\n"
	}

	gbkEncoder := simplifiedchinese.GBK.NewEncoder()
	gbkBytes, _, err := transform.Bytes(gbkEncoder, []byte(text))
	if err != nil {
		return "", fmt.Errorf("failed to encode text to GBK: %w", err)
	}
	return base64.StdEncoding.EncodeToString(gbkBytes), nil
}

func DecodeGBKBase64ToText(encoded string) (string, error) {
	gbkBytes, err := base64.StdEncoding.DecodeString(encoded)
	if err != nil {
		return "", fmt.Errorf("failed to decode base64: %w", err)
	}

	gbkDecoder := simplifiedchinese.GBK.NewDecoder()
	utf8Bytes, _, err := transform.Bytes(gbkDecoder, gbkBytes)
	if err != nil {
		return "", fmt.Errorf("failed to decode GBK to UTF-8: %w", err)
	}

	return string(utf8Bytes), nil
}

const (
	MaxPrintWidth   = 384
	MaxCharsPerLine = 32
)

func TruncateText(text string, maxLines int) string {
	if maxLines <= 0 {
		maxLines = 10
	}

	lines := strings.Split(text, "\n")
	if len(lines) <= maxLines {
		return text
	}

	return strings.Join(lines[:maxLines], "\n") + "\n..."
}

func WrapText(text string, maxWidth int) string {
	if maxWidth <= 0 {
		maxWidth = MaxCharsPerLine
	}

	var result strings.Builder
	runes := []rune(text)

	for i := 0; i < len(runes); i += maxWidth {
		end := i + maxWidth
		if end > len(runes) {
			end = len(runes)
		}
		result.WriteString(string(runes[i:end]))
		if end < len(runes) {
			result.WriteString("\n")
		}
	}

	return result.String()
}

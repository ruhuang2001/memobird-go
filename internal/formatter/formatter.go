package formatter

import (
	"encoding/base64"
	"fmt"

	"golang.org/x/text/encoding/simplifiedchinese"
	"golang.org/x/text/transform"
)

// EncodeHTMLToGBKBase64 encodes HTML for the printpaperFromHtml API.
func EncodeHTMLToGBKBase64(html string) (string, error) {
	return encodeGBKBase64(html, false)
}

func encodeGBKBase64(input string, appendTrailingNewline bool) (string, error) {
	if appendTrailingNewline {
		input += "\n"
	}

	gbkBytes, _, err := transform.Bytes(simplifiedchinese.GBK.NewEncoder(), []byte(input))
	if err != nil {
		return "", fmt.Errorf("failed to encode GBK payload: %w", err)
	}

	return base64.StdEncoding.EncodeToString(gbkBytes), nil
}

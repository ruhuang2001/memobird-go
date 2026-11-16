package formatter

import (
	"encoding/base64"
	"testing"

	"golang.org/x/text/encoding/simplifiedchinese"
	"golang.org/x/text/transform"
)

func TestEncodeHTMLToGBKBase64RoundTrip(t *testing.T) {
	html := "<div>中文测试 ABC 123</div>"

	encoded, err := EncodeHTMLToGBKBase64(html)
	if err != nil {
		t.Fatalf("EncodeHTMLToGBKBase64() error = %v", err)
	}

	decoded, err := decodeGBKBase64(encoded)
	if err != nil {
		t.Fatalf("decodeGBKBase64() error = %v", err)
	}

	if decoded != html {
		t.Fatalf("decoded payload = %q, want %q", decoded, html)
	}
}

func TestEncodeHTMLToGBKBase64DoesNotAppendNewline(t *testing.T) {
	html := "<p>line</p>"

	encoded, err := EncodeHTMLToGBKBase64(html)
	if err != nil {
		t.Fatalf("EncodeHTMLToGBKBase64() error = %v", err)
	}

	decoded, err := decodeGBKBase64(encoded)
	if err != nil {
		t.Fatalf("decodeGBKBase64() error = %v", err)
	}

	if decoded != html {
		t.Fatalf("decoded payload = %q, want %q", decoded, html)
	}
}

func TestEncodeHTMLToGBKBase64RejectsUnencodableCharacters(t *testing.T) {
	if _, err := EncodeHTMLToGBKBase64("<p>emoji 😀</p>"); err == nil {
		t.Fatal("EncodeHTMLToGBKBase64() error = nil, want encoding error")
	}
}

func decodeGBKBase64(encoded string) (string, error) {
	tBytes, err := base64.StdEncoding.DecodeString(encoded)
	if err != nil {
		return "", err
	}

	decoded, _, err := transform.Bytes(simplifiedchinese.GBK.NewDecoder(), tBytes)
	if err != nil {
		return "", err
	}

	return string(decoded), nil
}

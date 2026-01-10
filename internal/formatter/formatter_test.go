package formatter

import (
	"strings"
	"testing"
)

func TestEncodeTextToGBKBase64(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		wantErr bool
	}{
		{
			name:    "simple ASCII text",
			input:   "hello",
			wantErr: false,
		},
		{
			name:    "Chinese text",
			input:   "你好世界",
			wantErr: false,
		},
		{
			name:    "mixed text",
			input:   "Hello 你好",
			wantErr: false,
		},
		{
			name:    "text with newline",
			input:   "hello\n",
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := EncodeTextToGBKBase64(tt.input)
			if (err != nil) != tt.wantErr {
				t.Errorf("EncodeTextToGBKBase64() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if !tt.wantErr && result == "" {
				t.Error("EncodeTextToGBKBase64() returned empty string")
			}
		})
	}
}

func TestDecodeGBKBase64ToText(t *testing.T) {
	original := "你好世界"
	encoded, err := EncodeTextToGBKBase64(original)
	if err != nil {
		t.Fatalf("EncodeTextToGBKBase64() error = %v", err)
	}

	decoded, err := DecodeGBKBase64ToText(encoded)
	if err != nil {
		t.Fatalf("DecodeGBKBase64ToText() error = %v", err)
	}

	expected := original + "\n"
	if decoded != expected {
		t.Errorf("DecodeGBKBase64ToText() = %q, want %q", decoded, expected)
	}
}

func TestTruncateText(t *testing.T) {
	tests := []struct {
		name     string
		text     string
		maxLines int
		want     string
	}{
		{
			name:     "within limit",
			text:     "line1\nline2\nline3",
			maxLines: 5,
			want:     "line1\nline2\nline3",
		},
		{
			name:     "exceed limit",
			text:     "line1\nline2\nline3\nline4\nline5",
			maxLines: 3,
			want:     "line1\nline2\nline3\n...",
		},
		{
			name:     "default max lines",
			text:     "line1\nline2",
			maxLines: 0,
			want:     "line1\nline2",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := TruncateText(tt.text, tt.maxLines)
			if got != tt.want {
				t.Errorf("TruncateText() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestWrapText(t *testing.T) {
	tests := []struct {
		name     string
		text     string
		maxWidth int
		want     string
	}{
		{
			name:     "within width",
			text:     "hello",
			maxWidth: 10,
			want:     "hello",
		},
		{
			name:     "needs wrapping",
			text:     "hello world",
			maxWidth: 5,
			want:     "hello\n worl\nd",
		},
		{
			name:     "default width",
			text:     "short",
			maxWidth: 0,
			want:     "short",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := WrapText(tt.text, tt.maxWidth)
			if got != tt.want {
				t.Errorf("WrapText() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestEncodeDecodeRoundTrip(t *testing.T) {
	testCases := []string{
		"Hello, World!",
		"你好，世界！",
		"Test 123 测试",
		"Special chars: @#$%",
	}

	for _, original := range testCases {
		encoded, err := EncodeTextToGBKBase64(original)
		if err != nil {
			t.Errorf("EncodeTextToGBKBase64(%q) error = %v", original, err)
			continue
		}

		decoded, err := DecodeGBKBase64ToText(encoded)
		if err != nil {
			t.Errorf("DecodeGBKBase64ToText() error = %v", err)
			continue
		}

		expected := original
		if !strings.HasSuffix(original, "\n") {
			expected = original + "\n"
		}
		if decoded != expected {
			t.Errorf("Round trip failed: got %q, want %q", decoded, expected)
		}
	}
}

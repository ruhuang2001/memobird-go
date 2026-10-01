package renderer

import "github.com/ruhuang2001/memobird-go/internal/printutil"

// Image-processing constants are retained here for API compatibility.
const (
	TargetWidth                   = printutil.TargetWidth
	MaxEncodedImageLength         = printutil.MaxEncodedImageLength
	MaxDecodedImageBytes          = printutil.MaxDecodedImageBytes
	MaxSourceImagePixels          = printutil.MaxSourceImagePixels
	TrailingWhiteRowLumaThreshold = printutil.TrailingWhiteRowLumaThreshold
)

// ProcessImageForPrint resizes a base64 PNG to 384px width and makes it monochrome.
func ProcessImageForPrint(imgBase64 string) (string, error) {
	return printutil.ProcessImageForPrint(imgBase64)
}

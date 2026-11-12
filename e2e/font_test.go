package e2e

import (
	"bytes"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"image/png"
	"os"
	"path/filepath"
	"runtime"
	"testing"
	"time"

	"github.com/ruhuang2001/memobird-go/textrender"
	"golang.org/x/image/font/gofont/gobold"
	"golang.org/x/image/font/gofont/goregular"
)

// Failure cases were recorded in README.md before the dependency upgrade.
func TestE2EFontRendering(t *testing.T) {
	if os.Getenv("MEMOBIRD_E2E") != "1" {
		t.Skip("run make e2e")
	}
	dir := os.Getenv("MEMOBIRD_E2E_ARTIFACTS")
	if dir == "" {
		dir = t.TempDir()
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	type observation struct {
		Name           string
		Error          string
		Width, Height  int
		PixelSHA256    string
		ElapsedNS      int64
		AllocatedBytes uint64
	}
	var report []observation
	for _, tc := range []struct {
		name    string
		font    []byte
		invalid bool
	}{
		{"embedded", nil, false},
		{"custom-regular", goregular.TTF, false},
		{"custom-bold", gobold.TTF, false},
		{"invalid", []byte("not-a-font"), true},
		{"truncated", goregular.TTF[:100], true},
		{"recovery", nil, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var before, after runtime.MemStats
			runtime.ReadMemStats(&before)
			started := time.Now()
			data, err := textrender.RenderPNG("Font regression: Hello 123!\nSecond line", textrender.Options{FontData: tc.font})
			elapsed := time.Since(started)
			runtime.ReadMemStats(&after)
			obs := observation{Name: tc.name, ElapsedNS: elapsed.Nanoseconds(), AllocatedBytes: after.TotalAlloc - before.TotalAlloc}
			if tc.invalid {
				if err == nil {
					t.Fatal("invalid font unexpectedly rendered")
				}
				obs.Error = err.Error()
			} else {
				if err != nil {
					t.Fatal(err)
				}
				img, err := png.Decode(bytes.NewReader(data))
				if err != nil {
					t.Fatal(err)
				}
				obs.Width, obs.Height = img.Bounds().Dx(), img.Bounds().Dy()
				if obs.Width != 384 || obs.Height <= 32 {
					t.Fatalf("unexpected bounds %v", img.Bounds())
				}
				pixels := make([]byte, 0, obs.Width*obs.Height*8)
				for y := range obs.Height {
					for x := range obs.Width {
						r, g, b, a := img.At(x, y).RGBA()
						for _, v := range []uint32{r, g, b, a} {
							pixels = append(pixels, byte(v>>8), byte(v))
						}
					}
				}
				obs.PixelSHA256 = fmt.Sprintf("%x", sha256.Sum256(pixels))
				if tc.name == "custom-regular" || tc.name == "recovery" {
					if len(report) == 0 || obs.PixelSHA256 != report[0].PixelSHA256 {
						t.Fatal("default and explicit trusted font differ")
					}
				}
				if err := os.WriteFile(filepath.Join(dir, "font-"+tc.name+".png"), data, 0o644); err != nil {
					t.Fatal(err)
				}
			}
			report = append(report, obs)
		})
	}
	data, err := json.MarshalIndent(report, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "font-report.json"), data, 0o644); err != nil {
		t.Fatal(err)
	}
}

package renderer

import (
	"context"
	"encoding/base64"
	"fmt"
	"net/url"
	"sync"
	"time"

	"github.com/chromedp/cdproto/page"
	"github.com/chromedp/chromedp"
)

const (
	// PrinterWidth is the Memobird thermal printer capture width in pixels.
	PrinterWidth = 400

	// MaxRenderHeight is the maximum allowed page height for rendering.
	MaxRenderHeight = 2000

	// MaxRenderPixels is the maximum allowed total page pixels for rendering.
	MaxRenderPixels = PrinterWidth * MaxRenderHeight
)

const (
	urlRendererKind  = "url"
	htmlRendererKind = "html"
)

type renderBounds struct {
	Width  int `json:"width"`
	Height int `json:"height"`
}

type renderSegment struct {
	OffsetY int
	Height  int
}

// Renderer renders web content to images using headless Chrome.
type Renderer struct {
	timeout     time.Duration
	renderDelay time.Duration

	mu          sync.Mutex
	urlSession  *browserSession
	htmlSession *browserSession
	closed      bool
}

type browserSession struct {
	allocCtx      context.Context
	allocCancel   context.CancelFunc
	browserCtx    context.Context
	browserCancel context.CancelFunc
}

// New creates a renderer with the specified timeout.
func New(timeout time.Duration) *Renderer {
	if timeout == 0 {
		timeout = 30 * time.Second
	}
	return &Renderer{timeout: timeout, renderDelay: 500 * time.Millisecond}
}

// NewWithOptions creates a renderer with custom timeout and render delay.
func NewWithOptions(timeout, renderDelay time.Duration) *Renderer {
	if timeout == 0 {
		timeout = 30 * time.Second
	}
	if renderDelay == 0 {
		renderDelay = 500 * time.Millisecond
	}
	return &Renderer{timeout: timeout, renderDelay: renderDelay}
}

// Close releases background browser resources used by the renderer.
func (r *Renderer) Close() {
	r.mu.Lock()
	defer r.mu.Unlock()

	if r.closed {
		return
	}

	r.closeSession(&r.urlSession)
	r.closeSession(&r.htmlSession)
	r.closed = true
}

func (r *Renderer) closeSession(session **browserSession) {
	if *session == nil {
		return
	}

	(*session).browserCancel()
	(*session).allocCancel()
	*session = nil
}

func (r *Renderer) getBrowserContext(kind string) (context.Context, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	if r.closed {
		return nil, fmt.Errorf("renderer is closed")
	}

	sessionPtr, opts, err := r.sessionForKind(kind)
	if err != nil {
		return nil, err
	}

	if *sessionPtr == nil {
		*sessionPtr = newBrowserSession(opts)
	}

	return (*sessionPtr).browserCtx, nil
}

func (r *Renderer) sessionForKind(kind string) (**browserSession, []chromedp.ExecAllocatorOption, error) {
	baseOpts := []chromedp.ExecAllocatorOption{
		chromedp.Flag("headless", true),
		chromedp.Flag("disable-gpu", true),
		chromedp.Flag("no-sandbox", true),
		chromedp.WindowSize(PrinterWidth, 800),
	}

	switch kind {
	case urlRendererKind:
		opts := append(chromedp.DefaultExecAllocatorOptions[:], baseOpts...)
		opts = append(opts, chromedp.Flag("force-device-scale-factor", "2"))
		return &r.urlSession, opts, nil
	case htmlRendererKind:
		opts := append(chromedp.DefaultExecAllocatorOptions[:], baseOpts...)
		return &r.htmlSession, opts, nil
	default:
		return nil, nil, fmt.Errorf("unknown renderer kind: %s", kind)
	}
}

func newBrowserSession(opts []chromedp.ExecAllocatorOption) *browserSession {
	allocCtx, allocCancel := chromedp.NewExecAllocator(context.Background(), opts...)
	browserCtx, browserCancel := chromedp.NewContext(allocCtx)

	return &browserSession{
		allocCtx:      allocCtx,
		allocCancel:   allocCancel,
		browserCtx:    browserCtx,
		browserCancel: browserCancel,
	}
}

func (r *Renderer) newTaskContext(parent context.Context, requestCtx context.Context) (context.Context, context.CancelFunc) {
	baseCtx, baseCancel := context.WithCancel(parent)

	go func() {
		select {
		case <-requestCtx.Done():
			baseCancel()
		case <-baseCtx.Done():
		}
	}()

	timeoutCtx, timeoutCancel := context.WithTimeout(baseCtx, r.timeout)
	cancel := func() {
		timeoutCancel()
		baseCancel()
	}
	return timeoutCtx, cancel
}

// ValidateURL checks if a URL has a valid format and safe protocol.
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

func validateRenderBounds(width, height int) error {
	if width <= 0 || height <= 0 {
		return fmt.Errorf("invalid render bounds: %dx%d", width, height)
	}
	if height > MaxRenderHeight {
		return fmt.Errorf("render height %dpx exceeds max %dpx", height, MaxRenderHeight)
	}

	pixelCount := int64(width) * int64(height)
	if pixelCount > MaxRenderPixels {
		return fmt.Errorf("render area %dpx exceeds max %dpx (%dx%d)", pixelCount, MaxRenderPixels, width, height)
	}
	return nil
}

func validateMeasuredBounds(bounds renderBounds) error {
	if bounds.Width <= 0 || bounds.Height <= 0 {
		return fmt.Errorf("invalid render bounds: %dx%d", bounds.Width, bounds.Height)
	}
	if bounds.Width > MaxRenderPixels {
		return fmt.Errorf("render width %dpx exceeds max supported width %dpx", bounds.Width, MaxRenderPixels)
	}
	return nil
}

func segmentHeightForWidth(width int) (int, error) {
	if width <= 0 {
		return 0, fmt.Errorf("invalid render width: %d", width)
	}

	maxByPixels := MaxRenderPixels / width
	if maxByPixels <= 0 {
		return 0, fmt.Errorf("render width %dpx exceeds max supported width %dpx", width, MaxRenderPixels)
	}

	maxByProcessed := (MaxRenderHeight * width) / TargetWidth
	if maxByProcessed <= 0 {
		return 0, fmt.Errorf("render width %dpx is too small for processed output", width)
	}

	segmentHeight := minInt(MaxRenderHeight, maxByPixels)
	segmentHeight = minInt(segmentHeight, maxByProcessed)
	if segmentHeight <= 0 {
		return 0, fmt.Errorf("invalid segment height for width %d", width)
	}

	return segmentHeight, nil
}

func splitRenderBounds(bounds renderBounds) ([]renderSegment, error) {
	if err := validateMeasuredBounds(bounds); err != nil {
		return nil, err
	}

	segmentHeight, err := segmentHeightForWidth(bounds.Width)
	if err != nil {
		return nil, err
	}

	segments := make([]renderSegment, 0, (bounds.Height+segmentHeight-1)/segmentHeight)
	for offset := 0; offset < bounds.Height; offset += segmentHeight {
		height := minInt(segmentHeight, bounds.Height-offset)
		if err := validateRenderBounds(bounds.Width, height); err != nil {
			return nil, err
		}
		segments = append(segments, renderSegment{OffsetY: offset, Height: height})
	}

	return segments, nil
}

func measureCurrentPageBounds(ctx context.Context) (renderBounds, error) {
	var bounds renderBounds
	err := chromedp.Run(ctx, chromedp.Evaluate(`(() => {
		const doc = document.documentElement;
		const body = document.body;
		const width = Math.ceil(Math.max(
			doc ? doc.scrollWidth : 0,
			doc ? doc.offsetWidth : 0,
			doc ? doc.clientWidth : 0,
			body ? body.scrollWidth : 0,
			body ? body.offsetWidth : 0,
			body ? body.clientWidth : 0,
			window.innerWidth || 0
		));
		const height = Math.ceil(Math.max(
			doc ? doc.scrollHeight : 0,
			doc ? doc.offsetHeight : 0,
			doc ? doc.clientHeight : 0,
			body ? body.scrollHeight : 0,
			body ? body.offsetHeight : 0,
			body ? body.clientHeight : 0,
			window.innerHeight || 0
		));
		return { width, height };
	})()`, &bounds))
	if err != nil {
		return renderBounds{}, fmt.Errorf("failed to evaluate page bounds: %w", err)
	}

	return bounds, nil
}

func captureSegment(ctx context.Context, width int, segment renderSegment) (string, error) {
	clip := &page.Viewport{X: 0, Y: float64(segment.OffsetY), Width: float64(width), Height: float64(segment.Height), Scale: 1}

	var data []byte
	err := chromedp.Run(ctx, chromedp.ActionFunc(func(ctx context.Context) error {
		var err error
		data, err = page.CaptureScreenshot().WithFormat(page.CaptureScreenshotFormatPng).WithFromSurface(true).WithClip(clip).Do(ctx)
		return err
	}))
	if err != nil {
		return "", fmt.Errorf("failed to capture page segment at y=%d height=%d: %w", segment.OffsetY, segment.Height, err)
	}

	return base64.StdEncoding.EncodeToString(data), nil
}

func (r *Renderer) renderPageImages(ctx context.Context, kind string, setup chromedp.Tasks) ([]string, error) {
	browserCtx, err := r.getBrowserContext(kind)
	if err != nil {
		return nil, err
	}

	tabCtx, tabCancel := chromedp.NewContext(browserCtx)
	defer tabCancel()

	taskCtx, taskCancel := r.newTaskContext(tabCtx, ctx)
	defer taskCancel()

	initialTasks := append(chromedp.Tasks{}, setup...)
	initialTasks = append(initialTasks, chromedp.Sleep(r.renderDelay))
	if err := chromedp.Run(taskCtx, initialTasks...); err != nil {
		return nil, err
	}

	bounds, err := measureCurrentPageBounds(taskCtx)
	if err != nil {
		return nil, err
	}

	segments, err := splitRenderBounds(bounds)
	if err != nil {
		return nil, err
	}

	images := make([]string, 0, len(segments))
	for _, segment := range segments {
		imgBase64, err := captureSegment(taskCtx, bounds.Width, segment)
		if err != nil {
			return nil, err
		}
		images = append(images, imgBase64)
	}

	return images, nil
}

func singleImageResult(images []string) (string, error) {
	switch len(images) {
	case 0:
		return "", fmt.Errorf("renderer produced no images")
	case 1:
		return images[0], nil
	default:
		return "", fmt.Errorf("render requires pagination (%d pages)", len(images))
	}
}

// RenderURLToImage renders a webpage to a PNG image (base64 encoded).
func (r *Renderer) RenderURLToImage(ctx context.Context, pageURL string) (string, error) {
	images, err := r.RenderURLToImages(ctx, pageURL)
	if err != nil {
		return "", err
	}

	imgBase64, err := singleImageResult(images)
	if err != nil {
		return "", fmt.Errorf("failed to render page: %w", err)
	}
	return imgBase64, nil
}

// RenderURLToImages renders a webpage to one or more PNG images (base64 encoded).
func (r *Renderer) RenderURLToImages(ctx context.Context, pageURL string) ([]string, error) {
	fontCSS := `
		* {
			font-size: 24px !important;
			line-height: 1.6 !important;
		}
		html, body {
			margin: 0 !important;
			padding: 2px !important;
		}
		body, p, div, span, li, td, th, a, label {
			font-size: 24px !important;
		}
		h1 { font-size: 32px !important; }
		h2 { font-size: 28px !important; }
		h3 { font-size: 26px !important; }
		small { font-size: 20px !important; }
	`

	images, err := r.renderPageImages(ctx, urlRendererKind, chromedp.Tasks{
		chromedp.Navigate(pageURL),
		chromedp.WaitReady("body"),
		chromedp.ActionFunc(func(ctx context.Context) error {
			script := fmt.Sprintf(`
				var style = document.createElement('style');
				style.textContent = %q;
				document.head.appendChild(style);
			`, fontCSS)
			return chromedp.Evaluate(script, nil).Do(ctx)
		}),
	})
	if err != nil {
		return nil, fmt.Errorf("failed to render page: %w", err)
	}
	return images, nil
}

// RenderHTMLToImage renders HTML content to a PNG image (base64 encoded).
func (r *Renderer) RenderHTMLToImage(ctx context.Context, html string) (string, error) {
	images, err := r.RenderHTMLToImages(ctx, html)
	if err != nil {
		return "", err
	}

	imgBase64, err := singleImageResult(images)
	if err != nil {
		return "", fmt.Errorf("failed to render HTML: %w", err)
	}
	return imgBase64, nil
}

// RenderHTMLToImages renders HTML content to one or more PNG images (base64 encoded).
func (r *Renderer) RenderHTMLToImages(ctx context.Context, html string) ([]string, error) {
	wrappedHTML := fmt.Sprintf(`
	<!DOCTYPE html>
	<html>
	<head>
	<meta charset="UTF-8">
	<style>
	body {
		width: %dpx;
		margin: 0;
		padding: 8px;
		box-sizing: border-box;
		font-family: "PingFang SC", "Microsoft YaHei", sans-serif;
		font-size: 12px;
		line-height: 1.4;
		background: white;
		color: black;
	}
	img { max-width: 100%%; height: auto; }
	</style>
	</head>
	<body>%s</body>
	</html>`, PrinterWidth-16, html)

	images, err := r.renderPageImages(ctx, htmlRendererKind, chromedp.Tasks{
		chromedp.Navigate("about:blank"),
		chromedp.ActionFunc(func(ctx context.Context) error {
			return chromedp.Evaluate(fmt.Sprintf(`document.documentElement.innerHTML = %q`, wrappedHTML), nil).Do(ctx)
		}),
	})
	if err != nil {
		return nil, fmt.Errorf("failed to render HTML: %w", err)
	}
	return images, nil
}

func minInt(a, b int) int {
	if a < b {
		return a
	}
	return b
}

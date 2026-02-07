package renderer

import (
	"context"
	"encoding/base64"
	"fmt"
	"net/url"
	"sync"
	"time"

	"github.com/chromedp/chromedp"
)

const (
	// Memobird thermal printer width in pixels (slightly wider to use edge margins)
	PrinterWidth = 400
)

// Renderer handles rendering web content to images using headless Chrome.
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

// New creates a new Renderer with the specified timeout and optional render delay.
// Use NewWithOptions for more configuration options.
func New(timeout time.Duration) *Renderer {
	if timeout == 0 {
		timeout = 30 * time.Second
	}
	return &Renderer{
		timeout:     timeout,
		renderDelay: 500 * time.Millisecond,
	}
}

// NewWithOptions creates a new Renderer with custom options.
func NewWithOptions(timeout, renderDelay time.Duration) *Renderer {
	if timeout == 0 {
		timeout = 30 * time.Second
	}
	if renderDelay == 0 {
		renderDelay = 500 * time.Millisecond
	}
	return &Renderer{
		timeout:     timeout,
		renderDelay: renderDelay,
	}
}

// Close releases background browser resources used by the renderer.
func (r *Renderer) Close() {
	r.mu.Lock()
	defer r.mu.Unlock()

	if r.closed {
		return
	}

	if r.urlSession != nil {
		r.urlSession.browserCancel()
		r.urlSession.allocCancel()
		r.urlSession = nil
	}

	if r.htmlSession != nil {
		r.htmlSession.browserCancel()
		r.htmlSession.allocCancel()
		r.htmlSession = nil
	}

	r.closed = true
}

func (r *Renderer) getURLBrowserContext() (context.Context, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	if r.closed {
		return nil, fmt.Errorf("renderer is closed")
	}

	if r.urlSession == nil {
		opts := append(chromedp.DefaultExecAllocatorOptions[:],
			chromedp.Flag("headless", true),
			chromedp.Flag("disable-gpu", true),
			chromedp.Flag("no-sandbox", true),
			chromedp.Flag("force-device-scale-factor", "2"),
			chromedp.WindowSize(PrinterWidth, 800),
		)
		r.urlSession = newBrowserSession(opts)
	}

	return r.urlSession.browserCtx, nil
}

func (r *Renderer) getHTMLBrowserContext() (context.Context, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	if r.closed {
		return nil, fmt.Errorf("renderer is closed")
	}

	if r.htmlSession == nil {
		opts := append(chromedp.DefaultExecAllocatorOptions[:],
			chromedp.Flag("headless", true),
			chromedp.Flag("disable-gpu", true),
			chromedp.Flag("no-sandbox", true),
			chromedp.WindowSize(PrinterWidth, 800),
		)
		r.htmlSession = newBrowserSession(opts)
	}

	return r.htmlSession.browserCtx, nil
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
// Only http and https schemes are allowed (blocks file://, ftp://, etc.)
func ValidateURL(pageURL string) error {
	parsedURL, err := url.Parse(pageURL)
	if err != nil {
		return fmt.Errorf("invalid URL format: %w", err)
	}

	// Only allow http and https schemes
	if parsedURL.Scheme != "http" && parsedURL.Scheme != "https" {
		return fmt.Errorf("unsupported URL scheme: %s (only http/https allowed)", parsedURL.Scheme)
	}

	return nil
}

// RenderURLToImage renders a webpage to a PNG image (base64 encoded)
// Uses 2x scale for sharper text on thermal printers
func (r *Renderer) RenderURLToImage(ctx context.Context, pageURL string) (string, error) {
	browserCtx, err := r.getURLBrowserContext()
	if err != nil {
		return "", err
	}

	tabCtx, tabCancel := chromedp.NewContext(browserCtx)
	defer tabCancel()

	taskCtx, taskCancel := r.newTaskContext(tabCtx, ctx)
	defer taskCancel()

	// Inject CSS to increase font size and minimize margins for thermal printer
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

	var buf []byte
	err = chromedp.Run(taskCtx,
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
		chromedp.Sleep(r.renderDelay),
		chromedp.FullScreenshot(&buf, 100),
	)
	if err != nil {
		return "", fmt.Errorf("failed to render page: %w", err)
	}

	return base64.StdEncoding.EncodeToString(buf), nil
}

// RenderHTMLToImage renders HTML content to a PNG image (base64 encoded)
func (r *Renderer) RenderHTMLToImage(ctx context.Context, html string) (string, error) {
	browserCtx, err := r.getHTMLBrowserContext()
	if err != nil {
		return "", err
	}

	tabCtx, tabCancel := chromedp.NewContext(browserCtx)
	defer tabCancel()

	taskCtx, taskCancel := r.newTaskContext(tabCtx, ctx)
	defer taskCancel()

	// Wrap HTML with proper styling for thermal printer width
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

	var buf []byte
	err = chromedp.Run(taskCtx,
		chromedp.Navigate("about:blank"),
		chromedp.ActionFunc(func(ctx context.Context) error {
			return chromedp.Evaluate(fmt.Sprintf(`document.documentElement.innerHTML = %q`, wrappedHTML), nil).Do(ctx)
		}),
		chromedp.Sleep(r.renderDelay),
		chromedp.FullScreenshot(&buf, 100),
	)
	if err != nil {
		return "", fmt.Errorf("failed to render HTML: %w", err)
	}

	return base64.StdEncoding.EncodeToString(buf), nil
}

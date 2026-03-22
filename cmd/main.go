package main

import (
	"context"
	"flag"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/ruhuang2001/memobird-playground/internal/config"
	"github.com/ruhuang2001/memobird-playground/internal/memobird"
	"github.com/ruhuang2001/memobird-playground/internal/renderer"
	"github.com/ruhuang2001/memobird-playground/internal/storage"
)

var (
	configPath     = flag.String("config", "", "path to config file")
	bindUser       = flag.String("bind", "", "bind user identifier (run once to get user_id)")
	printURL       = flag.String("print-url", "", "print from URL via Memobird server-side rendering (modern pages may submit successfully but print blank; prefer -print-url-img)")
	printHTML      = flag.String("print-html", "", "print HTML via Memobird server-side rendering (may print blank on current devices/firmware; prefer -print-html-img)")
	printURLAsImg  = flag.String("print-url-img", "", "render URL locally as image and print (recommended)")
	printHTMLAsImg = flag.String("print-html-img", "", "render HTML locally as image and print (recommended)")
	showVersion    = flag.Bool("version", false, "show version")
)

var version = "dev"

type boundUser interface {
	GetUserID() int
}

type imagePrinter interface {
	PrintImage(ctx context.Context, imgBase64 string) (*memobird.PrintResponse, error)
	GetUserID() int
}

type imageRenderer interface {
	RenderURLToImage(ctx context.Context, pageURL string) (string, error)
	RenderHTMLToImage(ctx context.Context, html string) (string, error)
}

type pagedURLRenderer interface {
	RenderURLToImages(ctx context.Context, pageURL string) ([]string, error)
}

type pagedHTMLRenderer interface {
	RenderHTMLToImages(ctx context.Context, html string) ([]string, error)
}

type actionSpec struct {
	name       string
	value      string
	failureMsg string
	run        func() error
}

// main wires CLI flags, config loading, and command execution.
func main() {
	flag.Usage = func() {
		fmt.Fprintf(flag.CommandLine.Output(), "Usage of %s:\n", os.Args[0])
		flag.PrintDefaults()
		fmt.Fprintln(flag.CommandLine.Output())
		fmt.Fprintln(flag.CommandLine.Output(), "Notes:")
		fmt.Fprintln(flag.CommandLine.Output(), "  - -print-url and -print-html depend on Memobird server-side rendering.")
		fmt.Fprintln(flag.CommandLine.Output(), "  - Modern webpages may submit successfully but still print blank paper in those modes.")
		fmt.Fprintln(flag.CommandLine.Output(), "  - Prefer -print-url-img or -print-html-img for modern pages and reliable output.")
	}

	flag.Parse()

	if *showVersion {
		fmt.Printf("memobird-playground %s\n", version)
		os.Exit(0)
	}

	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelInfo,
	}))

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	cfg, err := config.Load(*configPath)
	if err != nil {
		logger.Error("failed to load config", "error", err)
		os.Exit(1)
	}

	store, err := storage.New(cfg.Storage.DBPath)
	if err != nil {
		logger.Error("failed to initialize storage", "error", err)
		os.Exit(1)
	}
	defer store.Close()

	client := memobird.NewClient(&cfg.Memobird)

	if err := initUserBinding(ctx, cfg, client, store, logger); err != nil {
		logger.Error("failed to initialize user binding", "error", err)
		os.Exit(1)
	}

	action, err := selectAction([]actionSpec{
		{
			name:       "bind",
			value:      *bindUser,
			failureMsg: "bind failed",
			run: func() error {
				return runBind(ctx, client, store, *bindUser, cfg.Memobird.DeviceID, logger)
			},
		},
		{
			name:       "print-url",
			value:      *printURL,
			failureMsg: "print from URL failed",
			run: func() error {
				return runPrintURL(ctx, client, *printURL, logger)
			},
		},
		{
			name:       "print-html",
			value:      *printHTML,
			failureMsg: "print from HTML failed",
			run: func() error {
				return runPrintHTML(ctx, client, *printHTML, logger)
			},
		},
		{
			name:       "print-url-img",
			value:      *printURLAsImg,
			failureMsg: "print URL as image failed",
			run: func() error {
				render := renderer.New(30 * time.Second)
				defer render.Close()
				return runPrintURLAsImage(ctx, client, render, *printURLAsImg, logger)
			},
		},
		{
			name:       "print-html-img",
			value:      *printHTMLAsImg,
			failureMsg: "print HTML as image failed",
			run: func() error {
				render := renderer.New(30 * time.Second)
				defer render.Close()
				return runPrintHTMLAsImage(ctx, client, render, *printHTMLAsImg, logger)
			},
		},
	})
	if err != nil {
		logger.Error("invalid command flags", "error", err)
		os.Exit(1)
	}
	if action == nil {
		flag.Usage()
		return
	}
	if err := action.run(); err != nil {
		logger.Error(action.failureMsg, "error", err)
		os.Exit(1)
	}
}

func selectAction(actions []actionSpec) (*actionSpec, error) {
	var selected *actionSpec
	for i := range actions {
		action := &actions[i]
		if action.value == "" {
			continue
		}
		if selected != nil {
			return nil, fmt.Errorf("flags -%s and -%s are mutually exclusive", selected.name, action.name)
		}
		selected = action
	}

	return selected, nil
}

// requireBoundUser ensures a valid user binding exists before print operations run.
func requireBoundUser(client boundUser) error {
	if client.GetUserID() == 0 {
		return fmt.Errorf("user_id not configured, run -bind first")
	}

	return nil
}

// initUserBinding initializes the user binding from config or storage.
func initUserBinding(ctx context.Context, cfg *config.Config, client *memobird.Client, store *storage.Storage, logger *slog.Logger) error {
	if cfg.Memobird.UserID > 0 {
		client.SetUserID(cfg.Memobird.UserID)
		return nil
	}

	userID, deviceID, err := store.GetUserBinding(ctx)
	if err != nil {
		return fmt.Errorf("failed to get user binding from storage: %w", err)
	}

	if userID > 0 && deviceID == cfg.Memobird.DeviceID {
		client.SetUserID(userID)
		logger.Info("loaded user binding from storage", "user_id", userID)
		return nil
	}

	logger.Warn("no user_id configured, run with -bind flag to bind device")
	return nil
}

// runBind executes the user binding flow and persists the result.
func runBind(ctx context.Context, client *memobird.Client, store *storage.Storage, userIdentifying, deviceID string, logger *slog.Logger) error {
	logger.Info("binding user", "user_identifying", userIdentifying)

	resp, err := client.BindUser(ctx, userIdentifying)
	if err != nil {
		return err
	}

	if err := store.SaveUserBinding(ctx, resp.UserID, deviceID); err != nil {
		logger.Warn("failed to save user binding", "error", err)
	}

	logger.Info("bind successful", "user_id", resp.UserID)
	fmt.Printf("\nBind successful!\n")
	fmt.Printf("Add this to your config.yaml:\n\n")
	fmt.Printf("memobird:\n")
	fmt.Printf("  user_id: %d\n\n", resp.UserID)

	return nil
}

// runPrintURL prints content from a web page URL.
func runPrintURL(ctx context.Context, client *memobird.Client, pageURL string, logger *slog.Logger) error {
	if err := requireBoundUser(client); err != nil {
		return err
	}

	logger.Warn("using Memobird server-side URL rendering; accepted jobs may still print blank pages on modern sites", "recommended_flag", "-print-url-img")
	logger.Info("printing from URL", "url", pageURL)

	if err := renderer.ValidateURL(pageURL); err != nil {
		return fmt.Errorf("URL validation failed: %w", err)
	}

	resp, err := client.PrintFromURL(ctx, pageURL)
	if err != nil {
		return err
	}

	logger.Info("print submitted", "content_id", resp.PrintContentID, "printed", resp.IsPrinted())
	fmt.Printf("Print submitted! Content ID: %d\n", resp.PrintContentID)

	return nil
}

// runPrintHTML prints HTML content directly.
func runPrintHTML(ctx context.Context, client *memobird.Client, html string, logger *slog.Logger) error {
	if err := requireBoundUser(client); err != nil {
		return err
	}

	logger.Warn("using Memobird server-side HTML rendering; keep HTML simple and prefer image mode for reliable output", "recommended_flag", "-print-html-img")
	logger.Info("printing HTML content", "length", len(html))

	resp, err := client.PrintFromHTML(ctx, html)
	if err != nil {
		return err
	}

	logger.Info("print submitted", "content_id", resp.PrintContentID, "printed", resp.IsPrinted())
	fmt.Printf("Print submitted! Content ID: %d\n", resp.PrintContentID)

	return nil
}

func renderImagePages(renderFn func() ([]string, error), renderErr string) ([]string, error) {
	imgBase64Pages, err := renderFn()
	if err != nil {
		return nil, fmt.Errorf("%s: %w", renderErr, err)
	}
	if len(imgBase64Pages) == 0 {
		return nil, fmt.Errorf("%s: renderer returned no pages", renderErr)
	}

	return imgBase64Pages, nil
}

func printRenderedImages(ctx context.Context, client imagePrinter, logger *slog.Logger, renderFn func() ([]string, error), renderErr string) error {
	imgBase64Pages, err := renderImagePages(renderFn, renderErr)
	if err != nil {
		return err
	}

	logger.Info("rendered image pages", "page_count", len(imgBase64Pages))

	for i, imgBase64 := range imgBase64Pages {
		page := i + 1
		total := len(imgBase64Pages)

		logger.Info("rendered image page", "page", page, "page_count", total, "base64_length", len(imgBase64))

		processedImg, err := renderer.ProcessImageForPrint(imgBase64)
		if err != nil {
			return fmt.Errorf("failed to process page %d/%d: %w", page, total, err)
		}

		logger.Info("processed image page", "page", page, "page_count", total, "base64_length", len(processedImg))

		resp, err := client.PrintImage(ctx, processedImg)
		if err != nil {
			return fmt.Errorf("failed to submit page %d/%d: %w", page, total, err)
		}

		logger.Info("print submitted", "page", page, "page_count", total, "content_id", resp.PrintContentID, "printed", resp.IsPrinted())
		fmt.Printf("Print submitted! Page %d/%d Content ID: %d\n", page, total, resp.PrintContentID)
	}
	return nil
}

// runPrintURLAsImage renders a web page to an image and prints it.
func runPrintURLAsImage(ctx context.Context, client imagePrinter, render imageRenderer, pageURL string, logger *slog.Logger) error {
	if err := requireBoundUser(client); err != nil {
		return err
	}

	logger.Info("rendering URL to image", "url", pageURL)

	if err := renderer.ValidateURL(pageURL); err != nil {
		return fmt.Errorf("URL validation failed: %w", err)
	}

	return printRenderedImages(ctx, client, logger, func() ([]string, error) {
		if paged, ok := render.(pagedURLRenderer); ok {
			return paged.RenderURLToImages(ctx, pageURL)
		}

		imgBase64, err := render.RenderURLToImage(ctx, pageURL)
		if err != nil {
			return nil, err
		}

		return []string{imgBase64}, nil
	}, "failed to render URL")
}

// runPrintHTMLAsImage renders HTML content to an image and prints it.
func runPrintHTMLAsImage(ctx context.Context, client imagePrinter, render imageRenderer, html string, logger *slog.Logger) error {
	if err := requireBoundUser(client); err != nil {
		return err
	}

	logger.Info("rendering HTML to image", "length", len(html))

	return printRenderedImages(ctx, client, logger, func() ([]string, error) {
		if paged, ok := render.(pagedHTMLRenderer); ok {
			return paged.RenderHTMLToImages(ctx, html)
		}

		imgBase64, err := render.RenderHTMLToImage(ctx, html)
		if err != nil {
			return nil, err
		}

		return []string{imgBase64}, nil
	}, "failed to render HTML")
}

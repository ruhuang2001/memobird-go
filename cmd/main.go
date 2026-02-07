package main

import (
	"context"
	"flag"
	"fmt"
	"log/slog"
	"os"
	"time"

	"github.com/ruhuang2001/memobird-playground/internal/config"
	"github.com/ruhuang2001/memobird-playground/internal/memobird"
	"github.com/ruhuang2001/memobird-playground/internal/renderer"
	"github.com/ruhuang2001/memobird-playground/internal/storage"
)

var (
	configPath     = flag.String("config", "", "path to config file")
	bindUser       = flag.String("bind", "", "bind user identifier (run once to get user_id)")
	printText      = flag.String("print-text", "", "print plain text immediately and exit (GBK text mode, experimental)")
	printURL       = flag.String("print-url", "", "print from URL immediately and exit")
	printHTML      = flag.String("print-html", "", "print HTML content immediately and exit")
	printURLAsImg  = flag.String("print-url-img", "", "render URL as image and print (for better text rendering)")
	printHTMLAsImg = flag.String("print-html-img", "", "render HTML as image and print")
	showVersion    = flag.Bool("version", false, "show version")
)

var version = "dev"

func main() {
	flag.Parse()

	if *showVersion {
		fmt.Printf("memobird-playground %s\n", version)
		os.Exit(0)
	}

	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelInfo,
	}))

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

	if err := initUserBinding(context.Background(), cfg, client, store, logger); err != nil {
		logger.Error("failed to initialize user binding", "error", err)
		os.Exit(1)
	}

	if *bindUser != "" {
		if err := runBind(context.Background(), client, store, *bindUser, cfg.Memobird.DeviceID, logger); err != nil {
			logger.Error("bind failed", "error", err)
			os.Exit(1)
		}
		return
	}

	if *printText != "" {
		if err := runPrintText(context.Background(), client, *printText, logger); err != nil {
			logger.Error("print text failed", "error", err)
			os.Exit(1)
		}
		return
	}

	if *printURL != "" {
		if err := runPrintURL(context.Background(), client, *printURL, logger); err != nil {
			logger.Error("print from URL failed", "error", err)
			os.Exit(1)
		}
		return
	}

	if *printHTML != "" {
		if err := runPrintHTML(context.Background(), client, *printHTML, logger); err != nil {
			logger.Error("print from HTML failed", "error", err)
			os.Exit(1)
		}
		return
	}

	render := renderer.New(30 * time.Second)
	defer render.Close()

	if *printURLAsImg != "" {
		if err := runPrintURLAsImage(context.Background(), client, render, *printURLAsImg, logger); err != nil {
			logger.Error("print URL as image failed", "error", err)
			os.Exit(1)
		}
		return
	}

	if *printHTMLAsImg != "" {
		if err := runPrintHTMLAsImage(context.Background(), client, render, *printHTMLAsImg, logger); err != nil {
			logger.Error("print HTML as image failed", "error", err)
			os.Exit(1)
		}
		return
	}

	flag.Usage()
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

// runPrintText prints plain text using the /home/printpaper endpoint.
func runPrintText(ctx context.Context, client *memobird.Client, text string, logger *slog.Logger) error {
	if client.GetUserID() == 0 {
		return fmt.Errorf("user_id not configured, run -bind first")
	}

	logger.Warn("print-text is experimental: some Memobird models/firmware may print blank paper", "suggestion", "use -print-html-img for stable Chinese output")

	logger.Info("printing text content", "length", len(text))

	resp, err := client.PrintText(ctx, text)
	if err != nil {
		return err
	}

	logger.Info("print submitted", "content_id", resp.PrintContentID, "printed", resp.IsPrinted())
	fmt.Printf("Print submitted! Content ID: %d\n", resp.PrintContentID)
	fmt.Printf("Note: -print-text may be unsupported on some devices and can result in blank output.\n")

	return nil
}

// runPrintURL prints content from a web page URL.
func runPrintURL(ctx context.Context, client *memobird.Client, pageURL string, logger *slog.Logger) error {
	if client.GetUserID() == 0 {
		return fmt.Errorf("user_id not configured, run -bind first")
	}

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
	if client.GetUserID() == 0 {
		return fmt.Errorf("user_id not configured, run -bind first")
	}

	logger.Info("printing HTML content", "length", len(html))

	resp, err := client.PrintFromHTML(ctx, html)
	if err != nil {
		return err
	}

	logger.Info("print submitted", "content_id", resp.PrintContentID, "printed", resp.IsPrinted())
	fmt.Printf("Print submitted! Content ID: %d\n", resp.PrintContentID)

	return nil
}

// runPrintURLAsImage renders a web page to an image and prints it.
func runPrintURLAsImage(ctx context.Context, client *memobird.Client, render *renderer.Renderer, pageURL string, logger *slog.Logger) error {
	if client.GetUserID() == 0 {
		return fmt.Errorf("user_id not configured, run -bind first")
	}

	logger.Info("rendering URL to image", "url", pageURL)

	if err := renderer.ValidateURL(pageURL); err != nil {
		return fmt.Errorf("URL validation failed: %w", err)
	}

	imgBase64, err := render.RenderURLToImage(ctx, pageURL)
	if err != nil {
		return fmt.Errorf("failed to render URL: %w", err)
	}

	logger.Info("rendered image", "base64_length", len(imgBase64))

	// Process image locally: resize to 384px and convert to high-contrast monochrome
	processedImg, err := renderer.ProcessImageForPrint(imgBase64)
	if err != nil {
		return fmt.Errorf("failed to process image: %w", err)
	}

	logger.Info("processed image", "base64_length", len(processedImg))

	resp, err := client.PrintImageProcessed(ctx, processedImg)
	if err != nil {
		return err
	}

	logger.Info("print submitted", "content_id", resp.PrintContentID, "printed", resp.IsPrinted())
	fmt.Printf("Print submitted! Content ID: %d\n", resp.PrintContentID)

	return nil
}

// runPrintHTMLAsImage renders HTML content to an image and prints it.
func runPrintHTMLAsImage(ctx context.Context, client *memobird.Client, render *renderer.Renderer, html string, logger *slog.Logger) error {
	if client.GetUserID() == 0 {
		return fmt.Errorf("user_id not configured, run -bind first")
	}

	logger.Info("rendering HTML to image", "length", len(html))

	imgBase64, err := render.RenderHTMLToImage(ctx, html)
	if err != nil {
		return fmt.Errorf("failed to render HTML: %w", err)
	}

	logger.Info("rendered image", "base64_length", len(imgBase64))

	// Process image locally: resize to 384px and convert to high-contrast monochrome
	processedImg, err := renderer.ProcessImageForPrint(imgBase64)
	if err != nil {
		return fmt.Errorf("failed to process image: %w", err)
	}

	logger.Info("processed image", "base64_length", len(processedImg))

	resp, err := client.PrintImageProcessed(ctx, processedImg)
	if err != nil {
		return err
	}

	logger.Info("print submitted", "content_id", resp.PrintContentID, "printed", resp.IsPrinted())
	fmt.Printf("Print submitted! Content ID: %d\n", resp.PrintContentID)

	return nil
}

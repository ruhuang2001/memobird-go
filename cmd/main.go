package main

import (
	"context"
	"flag"
	"fmt"
	"log/slog"
	"os"

	"github.com/ruhuang2001/memobird-playground/internal/config"
	"github.com/ruhuang2001/memobird-playground/internal/memobird"
	"github.com/ruhuang2001/memobird-playground/internal/storage"
)

var (
	configPath  = flag.String("config", "", "path to config file")
	bindUser    = flag.String("bind", "", "bind user identifier (run once to get user_id)")
	printURL    = flag.String("print-url", "", "print from URL immediately and exit")
	printHTML   = flag.String("print-html", "", "print HTML content immediately and exit")
	showVersion = flag.Bool("version", false, "show version")
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

	flag.Usage()
}

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

func runPrintURL(ctx context.Context, client *memobird.Client, pageURL string, logger *slog.Logger) error {
	if client.GetUserID() == 0 {
		return fmt.Errorf("user_id not configured, run -bind first")
	}

	logger.Info("printing from URL", "url", pageURL)

	resp, err := client.PrintFromURL(ctx, pageURL)
	if err != nil {
		return err
	}

	logger.Info("print submitted", "content_id", resp.PrintContentID, "printed", resp.IsPrinted())
	fmt.Printf("Print submitted! Content ID: %d\n", resp.PrintContentID)

	return nil
}

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

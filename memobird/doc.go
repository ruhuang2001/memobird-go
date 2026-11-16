// Package memobird is a client for the official Memobird thermal printer Web API.
//
// A Client binds a Memobird user to a device and prints content. Every print goes
// through an image, because the vendor's service-side renderers cannot print
// Chinese, so two entry points cover everything:
//
//   - PrintText renders text to an image with the textrender package and prints it.
//   - PrintImage prints an image the caller prepared.
//
// # Quick start
//
//	client, err := memobird.NewClient(memobird.Config{
//		AccessKey: os.Getenv("MEMOBIRD_ACCESS_KEY"),
//		DeviceID:  os.Getenv("MEMOBIRD_DEVICE_ID"),
//	})
//	if err != nil {
//		return err
//	}
//	if _, err := client.BindAndRemember(ctx, "user-identifying-string"); err != nil {
//		return err
//	}
//	if _, err := client.PrintText(ctx, "Hello, Memobird!", textrender.Options{}); err != nil {
//		return err
//	}
//
// # Fonts
//
// PrintText takes the same textrender.Options as the renderer, including
// FontData. The bundled font covers Latin only; pass a CJK face to print Chinese.
// Text in a script the font cannot draw returns ErrMissingGlyphs rather than
// printing a blank box for every character.
//
// # Concurrency
//
// A Client is safe for concurrent use once returned by NewClient. SetUserID,
// SetUserAgent, and the print methods may be called from multiple goroutines.
//
// # Errors
//
// Configuration and workflow failures are reported with the exported sentinel
// errors ErrUserIDNotConfigured, ErrUnknownPrintID, and ErrMissingGlyphs, so
// callers can test them with errors.Is. A request that the remote API rejects
// returns *APIError.
package memobird

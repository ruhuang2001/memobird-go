// Package memobird is a client for the official Memobird thermal printer Web API.
//
// A Client binds a Memobird user to a device and submits print jobs. Printed
// content can come from three paths:
//
//   - PrintURL and PrintHTML ask the Memobird service to render the page.
//   - PrintURLAsImages and PrintHTMLAsImages render locally with the renderer
//     package, then submit the resulting page images.
//   - PrintImage submits an already prepared base64 PNG, for example one
//     produced by the textrender package.
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
//	if _, err := client.PrintHTML(ctx, "<h1>Hello, Memobird!</h1>"); err != nil {
//		return err
//	}
//
// # Concurrency
//
// A Client is safe for concurrent use once returned by NewClient. SetUserID,
// SetUserAgent, and the print methods may be called from multiple goroutines.
//
// # Errors
//
// Configuration and workflow failures are reported with the exported sentinel
// errors ErrUserIDNotConfigured, ErrBindingStoreRequired, ErrRendererRequired,
// and ErrNoPagesRendered, so callers can test them with errors.Is. A request
// that the remote API rejects returns *APIError.
package memobird

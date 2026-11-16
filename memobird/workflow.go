package memobird

import (
	"context"
	"errors"
	"fmt"
	"net/url"
	"time"

	"github.com/ruhuang2001/memobird-go/formatter"
	"github.com/ruhuang2001/memobird-go/internal/printutil"
)

// BindAndRemember binds a user identifier and stores the returned user ID in the client.
func (c *Client) BindAndRemember(ctx context.Context, userIdentifying string) (*BindResponse, error) {
	resp, err := c.BindUser(ctx, userIdentifying)
	if err != nil {
		return nil, err
	}
	c.SetUserID(resp.UserID)
	return resp, nil
}

// PrintURL validates the URL and submits it for printing.
func (c *Client) PrintURL(ctx context.Context, pageURL string) (*PrintResponse, error) {
	if err := c.requireUserID(); err != nil {
		return nil, err
	}
	if err := printutil.ValidateURL(pageURL); err != nil {
		return nil, fmt.Errorf("URL validation failed: %w", err)
	}

	params := url.Values{}
	params.Set("memobirdID", c.deviceID)
	params.Set("userID", fmt.Sprintf("%d", c.GetUserID()))
	params.Set("printUrl", pageURL)

	return postFormJSON[PrintResponse](ctx, c, "/home/printpaperFromUrl", params, "print from URL")
}

// PrintHTML submits HTML for printing.
func (c *Client) PrintHTML(ctx context.Context, html string) (*PrintResponse, error) {
	if err := c.requireUserID(); err != nil {
		return nil, err
	}

	encoded, err := formatter.EncodeHTMLToGBKBase64(html)
	if err != nil {
		return nil, fmt.Errorf("failed to encode HTML: %w", err)
	}

	params := url.Values{}
	params.Set("memobirdID", c.deviceID)
	params.Set("userID", fmt.Sprintf("%d", c.GetUserID()))
	params.Set("printHtml", encoded)

	return postFormJSON[PrintResponse](ctx, c, "/home/printpaperFromHtml", params, "print from HTML")
}

// WaitOptions tunes WaitForPrint polling.
type WaitOptions struct {
	// Interval is the delay between status checks. Defaults to 1s.
	Interval time.Duration
	// Timeout bounds the whole wait. Defaults to 2m and is applied on top of ctx.
	Timeout time.Duration
}

func (o WaitOptions) normalized() WaitOptions {
	if o.Interval <= 0 {
		o.Interval = time.Second
	}
	if o.Timeout <= 0 {
		o.Timeout = 2 * time.Minute
	}
	return o
}

// ErrUnknownPrintID is returned when the API has no record of a print content
// ID, which happens when the ID was never issued by a successful print request.
var ErrUnknownPrintID = errors.New("memobird: unknown print content ID")

// WaitForPrint polls the print status until the job reaches a terminal state,
// opts.Timeout elapses, or ctx is canceled. It returns the last observed status.
//
// A device that is offline keeps the job pending in the cloud and prints it
// after it reconnects, so Timeout is the only bound on how long a caller waits;
// the default exists for that reason and should be raised deliberately rather
// than removed.
func (c *Client) WaitForPrint(ctx context.Context, printContentID int, opts WaitOptions) (*PrintStatusResponse, error) {
	if err := c.requireUserID(); err != nil {
		return nil, err
	}
	if printContentID <= 0 {
		return nil, fmt.Errorf("print content ID must be positive, got %d", printContentID)
	}

	opts = opts.normalized()
	ctx, cancel := context.WithTimeout(ctx, opts.Timeout)
	defer cancel()

	var last *PrintStatusResponse
	for {
		status, err := c.GetPrintStatus(ctx, printContentID)
		if err != nil {
			if errors.Is(err, context.DeadlineExceeded) || errors.Is(err, context.Canceled) {
				return last, fmt.Errorf("waiting for print %d: %w", printContentID, err)
			}
			return last, err
		}
		last = status
		if status.Terminal() {
			if status.IsUnknown() {
				return status, fmt.Errorf("%w: %d", ErrUnknownPrintID, printContentID)
			}
			return status, nil
		}

		if err := sleepContext(ctx, opts.Interval); err != nil {
			return last, fmt.Errorf("waiting for print %d: %w", printContentID, err)
		}
	}
}

// sleepContext waits for d or until ctx is done, whichever comes first.
func sleepContext(ctx context.Context, d time.Duration) error {
	if d <= 0 {
		return ctx.Err()
	}

	timer := time.NewTimer(d)
	defer timer.Stop()

	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return nil
	}
}

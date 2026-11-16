package memobird

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/ruhuang2001/memobird-go/textrender"
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

// ErrMissingGlyphs is returned when the font cannot draw some of the text, which
// would print a blank box in place of each character it lacks.
var ErrMissingGlyphs = errors.New("memobird: font does not cover the text")

// PrintText renders text to an image and prints it.
//
// Every print goes through an image because the vendor does not decode GBK, so
// Chinese sent as text or HTML reaches the paper as mojibake. opts are
// textrender.Options: the bundled font covers Latin only, so pass a CJK face as
// FontData for Chinese. Text the font cannot draw returns ErrMissingGlyphs
// rather than a blank box per character.
func (c *Client) PrintText(ctx context.Context, text string, opts textrender.Options) (*PrintResponse, error) {
	if err := c.requireUserID(); err != nil {
		return nil, err
	}

	covered, err := textrender.FontHasGlyphs(text, opts.FontData)
	if err != nil {
		return nil, err
	}
	if !covered {
		return nil, fmt.Errorf("%w: pass textrender.Options.FontData for this script", ErrMissingGlyphs)
	}

	imgBase64, err := textrender.RenderBase64PNG(text, opts)
	if err != nil {
		return nil, fmt.Errorf("failed to render text: %w", err)
	}

	return c.PrintImage(ctx, imgBase64)
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

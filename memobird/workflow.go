package memobird

import (
	"context"
	"fmt"
	"reflect"

	"github.com/ruhuang2001/memobird-go/renderer"
)

// BindingStore is the minimal persistence contract needed to remember and
// restore a bound Memobird user across process restarts.
type BindingStore interface {
	SaveUserBinding(ctx context.Context, userID int, deviceID string) error
	GetUserBinding(ctx context.Context) (userID int, deviceID string, err error)
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

func (c *Client) requireBoundUser() error {
	return c.requireUserID()
}

// BindAndRemember binds a user identifier and stores the returned user ID in the client.
func (c *Client) BindAndRemember(ctx context.Context, userIdentifying string) (*BindResponse, error) {
	resp, err := c.BindUser(ctx, userIdentifying)
	if err != nil {
		return nil, err
	}
	c.SetUserID(resp.UserID)
	return resp, nil
}

// PersistUserBinding saves the currently configured user binding to a store.
func (c *Client) PersistUserBinding(ctx context.Context, store BindingStore) error {
	return c.persistUserBinding(ctx, store, c.GetUserID())
}

// RestoreUserBinding loads a previously saved binding into the client.
// It returns false when the store has no binding yet.
func (c *Client) RestoreUserBinding(ctx context.Context, store BindingStore) (bool, error) {
	if store == nil {
		return false, fmt.Errorf("binding store is required")
	}

	userID, deviceID, err := store.GetUserBinding(ctx)
	if err != nil {
		return false, fmt.Errorf("failed to load user binding: %w", err)
	}
	if userID == 0 {
		return false, nil
	}
	if deviceID != "" && deviceID != c.deviceID {
		return false, fmt.Errorf("stored binding belongs to device %q, client configured for %q", deviceID, c.deviceID)
	}

	c.SetUserID(userID)
	return true, nil
}

// BindAndPersist binds a user identifier, stores the resulting user ID, and
// updates the client for subsequent print requests.
func (c *Client) BindAndPersist(ctx context.Context, store BindingStore, userIdentifying string) (*BindResponse, error) {
	if store == nil {
		return nil, fmt.Errorf("binding store is required")
	}

	resp, err := c.BindAndRemember(ctx, userIdentifying)
	if err != nil {
		return nil, err
	}

	if err := c.persistUserBinding(ctx, store, resp.UserID); err != nil {
		return resp, fmt.Errorf("binding succeeded but persistence failed: %w", err)
	}

	return resp, nil
}

// PrintURL validates the URL and submits it using Memobird server-side rendering.
func (c *Client) PrintURL(ctx context.Context, pageURL string) (*PrintResponse, error) {
	if err := c.requireBoundUser(); err != nil {
		return nil, err
	}
	if err := renderer.ValidateURL(pageURL); err != nil {
		return nil, fmt.Errorf("URL validation failed: %w", err)
	}
	return c.PrintFromURL(ctx, pageURL)
}

// PrintHTML submits HTML using Memobird server-side rendering.
func (c *Client) PrintHTML(ctx context.Context, html string) (*PrintResponse, error) {
	if err := c.requireBoundUser(); err != nil {
		return nil, err
	}
	return c.PrintFromHTML(ctx, html)
}

// PrintURLAsImages renders a URL locally and submits each rendered page as an image.
func (c *Client) PrintURLAsImages(ctx context.Context, render imageRenderer, pageURL string) ([]*PrintResponse, error) {
	if err := c.requireBoundUser(); err != nil {
		return nil, err
	}
	if isNilImageRenderer(render) {
		return nil, fmt.Errorf("image renderer is required")
	}
	if err := renderer.ValidateURL(pageURL); err != nil {
		return nil, fmt.Errorf("URL validation failed: %w", err)
	}
	return c.printRenderedImages(ctx, func() ([]string, error) {
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

// PrintHTMLAsImages renders HTML locally and submits each rendered page as an image.
func (c *Client) PrintHTMLAsImages(ctx context.Context, render imageRenderer, html string) ([]*PrintResponse, error) {
	if err := c.requireBoundUser(); err != nil {
		return nil, err
	}
	if isNilImageRenderer(render) {
		return nil, fmt.Errorf("image renderer is required")
	}
	return c.printRenderedImages(ctx, func() ([]string, error) {
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

func isNilImageRenderer(render imageRenderer) bool {
	if render == nil {
		return true
	}

	value := reflect.ValueOf(render)
	switch value.Kind() {
	case reflect.Chan, reflect.Func, reflect.Interface, reflect.Map, reflect.Pointer, reflect.Slice:
		return value.IsNil()
	default:
		return false
	}
}

func (c *Client) persistUserBinding(ctx context.Context, store BindingStore, userID int) error {
	if store == nil {
		return fmt.Errorf("binding store is required")
	}
	if userID == 0 {
		return fmt.Errorf("user_id not configured")
	}

	if err := store.SaveUserBinding(ctx, userID, c.deviceID); err != nil {
		return fmt.Errorf("failed to persist user binding: %w", err)
	}

	return nil
}

func (c *Client) printRenderedImages(ctx context.Context, renderFn func() ([]string, error), renderErr string) ([]*PrintResponse, error) {
	imgBase64Pages, err := renderImagePages(renderFn, renderErr)
	if err != nil {
		return nil, err
	}

	responses := make([]*PrintResponse, 0, len(imgBase64Pages))
	for i, imgBase64 := range imgBase64Pages {
		processedImg, err := renderer.ProcessImageForPrint(imgBase64)
		if err != nil {
			return responses, fmt.Errorf("failed to process page %d/%d: %w", i+1, len(imgBase64Pages), err)
		}

		resp, err := c.PrintImage(ctx, processedImg)
		if err != nil {
			return responses, fmt.Errorf("failed to submit page %d/%d: %w", i+1, len(imgBase64Pages), err)
		}
		responses = append(responses, resp)
	}

	return responses, nil
}

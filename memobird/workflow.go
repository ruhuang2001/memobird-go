package memobird

import (
	"context"
	"fmt"

	"github.com/ruhuang2001/memobird-playground/renderer"
)

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
	if c.GetUserID() == 0 {
		return fmt.Errorf("user_id not configured")
	}
	return nil
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
	if err := renderer.ValidateURL(pageURL); err != nil {
		return nil, fmt.Errorf("URL validation failed: %w", err)
	}
	return c.printRenderedImages(ctx, render, func() ([]string, error) {
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
	return c.printRenderedImages(ctx, render, func() ([]string, error) {
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

func (c *Client) printRenderedImages(ctx context.Context, _ imageRenderer, renderFn func() ([]string, error), renderErr string) ([]*PrintResponse, error) {
	imgBase64Pages, err := renderImagePages(renderFn, renderErr)
	if err != nil {
		return nil, err
	}

	responses := make([]*PrintResponse, 0, len(imgBase64Pages))
	for i, imgBase64 := range imgBase64Pages {
		processedImg, err := renderer.ProcessImageForPrint(imgBase64)
		if err != nil {
			return nil, fmt.Errorf("failed to process page %d/%d: %w", i+1, len(imgBase64Pages), err)
		}

		resp, err := c.PrintImage(ctx, processedImg)
		if err != nil {
			return nil, fmt.Errorf("failed to submit page %d/%d: %w", i+1, len(imgBase64Pages), err)
		}
		responses = append(responses, resp)
	}

	return responses, nil
}

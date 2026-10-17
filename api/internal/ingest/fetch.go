package ingest

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"

	"example.com/rolescout/api/internal/crawl"
)

// ErrBoardNotFound means the source's board or page does not exist (404).
var ErrBoardNotFound = errors.New("board not found")

// FetchJSON GETs rawURL through the polite client and decodes a JSON body
// into dst. Temporary failures are marked Retryable; 404 is ErrBoardNotFound.
func FetchJSON(ctx context.Context, c *crawl.Client, rawURL string, dst any) error {
	res, err := c.Get(ctx, rawURL, "application/json", nil)
	if err := classifyFetchError(err); err != nil {
		return err
	}
	return decodeJSON(res.Body, dst)
}

// PostJSON sends body as JSON and decodes the JSON response into dst.
func PostJSON(ctx context.Context, c *crawl.Client, rawURL string, body, dst any) error {
	payload, err := json.Marshal(body)
	if err != nil {
		return err
	}
	res, err := c.Post(ctx, rawURL, "application/json", payload)
	if err := classifyFetchError(err); err != nil {
		return err
	}
	return decodeJSON(res.Body, dst)
}

// Fetch GETs rawURL and returns the body, with the same error mapping as
// FetchJSON.
func Fetch(ctx context.Context, c *crawl.Client, rawURL, accept string) ([]byte, error) {
	res, err := c.Get(ctx, rawURL, accept, nil)
	if err := classifyFetchError(err); err != nil {
		return nil, err
	}
	return res.Body, nil
}

func classifyFetchError(err error) error {
	if err == nil {
		return nil
	}
	var se *crawl.StatusError
	if errors.As(err, &se) && (se.StatusCode == http.StatusNotFound || se.StatusCode == http.StatusGone) {
		return fmt.Errorf("%w: %w", ErrBoardNotFound, err)
	}
	if crawl.IsTemporary(err) {
		return Retryable(err)
	}
	return err
}

func decodeJSON(body []byte, dst any) error {
	if err := json.Unmarshal(body, dst); err != nil {
		return fmt.Errorf("decode response: %w", err)
	}
	return nil
}

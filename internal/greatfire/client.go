// Package greatfire fetches blocked entries from the GreatFire Analyzer JSON API.
package greatfire

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"net/url"
	"strconv"
	"sync"
	"time"
)

const (
	// DefaultBaseURL is the GreatFire API endpoint listing tags.
	DefaultBaseURL = "https://en.greatfire.org/api/tags"
	// PageSize is the number of items requested per page.
	PageSize = 200
)

// Page is the subset of the API response used by this project.
type Page struct {
	Total int    `json:"total"`
	Items []Item `json:"items"`
}

// Item is a single blocked entry.
type Item struct {
	Registrable string `json:"registrable"`
}

// Client fetches all pages of blocked entries.
type Client struct {
	BaseURL    string
	HTTPClient *http.Client
	// Parallel is the maximum number of pages fetched at a time.
	Parallel int
	// Delay is the interval between starting pages within a batch.
	Delay time.Duration
	// Retries is the number of extra attempts for a failed page.
	Retries int
	// MaxPages limits the number of pages fetched; 0 means all pages.
	MaxPages int
}

// NewClient returns a Client with sensible defaults.
func NewClient(parallel int) *Client {
	return &Client{
		BaseURL:    DefaultBaseURL,
		HTTPClient: &http.Client{Timeout: 30 * time.Second},
		Parallel:   parallel,
		Delay:      100 * time.Millisecond,
		Retries:    3,
	}
}

// FetchAll returns the "registrable" value of every blocked item across all
// pages, or across the first c.MaxPages pages if c.MaxPages is positive.
func (c *Client) FetchAll(ctx context.Context) ([]string, error) {
	first, err := c.fetchPageWithRetry(ctx, 0)
	if err != nil {
		return nil, err
	}
	results := collect(first)

	pageCount := first.Total / PageSize
	if first.Total%PageSize != 0 {
		pageCount++
	}
	if c.MaxPages > 0 {
		pageCount = min(pageCount, c.MaxPages)
	}

	parallel := max(c.Parallel, 1)
	for start := 1; start < pageCount; {
		batch := make([]int, min(parallel, pageCount-start))
		for i := range batch {
			batch[i] = (start + i) * PageSize
		}
		pages, err := c.fetchBatch(ctx, batch)
		if err != nil {
			return nil, err
		}
		for _, p := range pages {
			results = append(results, collect(p)...)
		}
		start += len(batch)
	}
	return results, nil
}

// fetchBatch fetches the given offsets concurrently, starting each request
// c.Delay after the previous one.
func (c *Client) fetchBatch(ctx context.Context, offsets []int) ([]*Page, error) {
	pages := make([]*Page, len(offsets))
	errs := make([]error, len(offsets))
	var wg sync.WaitGroup
	for i, off := range offsets {
		if i > 0 {
			if err := sleep(ctx, c.Delay); err != nil {
				wg.Wait()
				return nil, err
			}
		}
		wg.Add(1)
		go func(i, off int) {
			defer wg.Done()
			pages[i], errs[i] = c.fetchPageWithRetry(ctx, off)
		}(i, off)
	}
	wg.Wait()
	for _, err := range errs {
		if err != nil {
			return nil, err
		}
	}
	return pages, nil
}

func (c *Client) fetchPageWithRetry(ctx context.Context, offset int) (*Page, error) {
	var err error
	for attempt := 0; attempt <= c.Retries; attempt++ {
		if attempt > 0 {
			if serr := sleep(ctx, time.Duration(attempt)*time.Second); serr != nil {
				return nil, serr
			}
		}
		var p *Page
		if p, err = c.fetchPage(ctx, offset); err == nil {
			return p, nil
		}
	}
	return nil, fmt.Errorf("fetch offset %d: %w", offset, err)
}

func (c *Client) fetchPage(ctx context.Context, offset int) (*Page, error) {
	u, err := url.Parse(c.BaseURL)
	if err != nil {
		return nil, err
	}
	q := u.Query()
	q.Set("status", "blocked")
	q.Set("limit", strconv.Itoa(PageSize))
	q.Set("offset", strconv.Itoa(offset))
	u.RawQuery = q.Encode()
	log.Printf("Crawling: %s", u)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u.String(), nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "application/json")
	resp, err := c.HTTPClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("unexpected status %s", resp.Status)
	}
	var p struct {
		Total *int   `json:"total"`
		Items []Item `json:"items"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&p); err != nil {
		return nil, fmt.Errorf("decode JSON: %w", err)
	}
	if p.Total == nil || *p.Total < 0 || p.Items == nil {
		return nil, fmt.Errorf("invalid page: expected a non-negative total and an items array")
	}
	if offset > 0 && offset >= *p.Total {
		return nil, fmt.Errorf("invalid page: offset %d is outside total %d", offset, *p.Total)
	}
	if want := min(PageSize, *p.Total-offset); len(p.Items) != want {
		return nil, fmt.Errorf("incomplete page: got %d items, want %d", len(p.Items), want)
	}
	return &Page{Total: *p.Total, Items: p.Items}, nil
}

func collect(p *Page) []string {
	out := make([]string, 0, len(p.Items))
	for _, it := range p.Items {
		out = append(out, it.Registrable)
	}
	return out
}

func sleep(ctx context.Context, d time.Duration) error {
	t := time.NewTimer(d)
	defer t.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-t.C:
		return nil
	}
}

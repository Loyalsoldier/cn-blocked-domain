package crawler

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"sync"
	"time"
)

const (
	endpoint = "https://en.greatfire.org/api/tags"
	pageSize = 200
)

type item struct {
	Registrable string `json:"registrable"`
}

type page struct {
	Total *int    `json:"total"`
	Items *[]item `json:"items"`
}

// Client fetches paginated data with a shared request rate limit.
type Client struct {
	http        *http.Client
	endpoint    string
	maxParallel int
	interval    time.Duration
	retryDelay  time.Duration
	mu          sync.Mutex
	lastRequest time.Time
}

func New(maxParallel int) (*Client, error) {
	if maxParallel < 1 {
		return nil, fmt.Errorf("max-parallel must be at least 1")
	}
	return &Client{
		http:        &http.Client{Timeout: 30 * time.Second},
		endpoint:    endpoint,
		maxParallel: maxParallel,
		interval:    100 * time.Millisecond,
		retryDelay:  time.Second,
	}, nil
}

func wait(ctx context.Context, delay time.Duration) error {
	timer := time.NewTimer(delay)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return ctx.Err()
	}
}

func (c *Client) pace(ctx context.Context) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	if err := wait(ctx, time.Until(c.lastRequest.Add(c.interval))); err != nil {
		return err
	}
	c.lastRequest = time.Now()
	return nil
}

func (c *Client) fetchPage(ctx context.Context, offset int) (page, error) {
	target, err := url.Parse(c.endpoint)
	if err != nil {
		return page{}, err
	}
	query := target.Query()
	query.Set("status", "blocked")
	query.Set("limit", strconv.Itoa(pageSize))
	query.Set("offset", strconv.Itoa(offset))
	target.RawQuery = query.Encode()

	for attempt := 0; attempt < 3; attempt++ {
		if attempt > 0 {
			if err := wait(ctx, c.retryDelay*time.Duration(1<<(attempt-1))); err != nil {
				return page{}, err
			}
		}
		if err := c.pace(ctx); err != nil {
			return page{}, err
		}
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, target.String(), nil)
		if err != nil {
			return page{}, err
		}
		req.Header.Set("Accept", "application/json")
		req.Header.Set("User-Agent", "cn-blocked-domain")
		resp, err := c.http.Do(req)
		if err != nil {
			if attempt < 2 && ctx.Err() == nil {
				continue
			}
			return page{}, fmt.Errorf("offset %d: %w", offset, err)
		}
		if resp.StatusCode != http.StatusOK {
			resp.Body.Close()
			if attempt < 2 && (resp.StatusCode == http.StatusTooManyRequests || resp.StatusCode >= 500) {
				continue
			}
			return page{}, fmt.Errorf("offset %d: HTTP %s", offset, resp.Status)
		}
		result, err := decodePage(resp.Body, offset)
		resp.Body.Close()
		return result, err
	}
	panic("unreachable")
}

func decodePage(body io.Reader, offset int) (page, error) {
	var result page
	decoder := json.NewDecoder(io.LimitReader(body, 8<<20))
	if err := decoder.Decode(&result); err != nil {
		return page{}, fmt.Errorf("offset %d: invalid JSON: %w", offset, err)
	}
	if err := decoder.Decode(new(any)); err != io.EOF {
		return page{}, fmt.Errorf("offset %d: unexpected data after JSON response", offset)
	}
	if result.Total == nil || *result.Total < 0 || result.Items == nil {
		return page{}, fmt.Errorf("offset %d: expected nonnegative total and an items array", offset)
	}
	expected := min(pageSize, max(0, *result.Total-offset))
	if len(*result.Items) != expected {
		return page{}, fmt.Errorf("offset %d: expected %d items, got %d", offset, expected, len(*result.Items))
	}
	return result, nil
}

// Fetch uses the first page's total to visit every offset, failing rather than
// returning partial results if any page cannot be retrieved.
func (c *Client) Fetch(ctx context.Context) ([]string, error) {
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
	first, err := c.fetchPage(ctx, 0)
	if err != nil {
		return nil, err
	}
	values := make([]string, 0, len(*first.Items))
	appendPage := func(p page) {
		for _, entry := range *p.Items {
			values = append(values, entry.Registrable)
		}
	}
	appendPage(first)
	remaining := 0
	if *first.Total > 0 {
		remaining = (*first.Total - 1) / pageSize
	}
	if remaining == 0 {
		return values, nil
	}

	type result struct {
		page page
		err  error
	}
	jobs := make(chan int)
	results := make(chan result)
	var workers sync.WaitGroup
	for i := 0; i < min(c.maxParallel, remaining); i++ {
		workers.Add(1)
		go func() {
			defer workers.Done()
			for offset := range jobs {
				p, err := c.fetchPage(ctx, offset)
				select {
				case results <- result{p, err}:
				case <-ctx.Done():
					return
				}
				if err != nil {
					return
				}
			}
		}()
	}
	go func() {
		defer close(jobs)
		for n := 1; n <= remaining; n++ {
			select {
			case jobs <- n * pageSize:
			case <-ctx.Done():
				return
			}
		}
	}()
	go func() {
		workers.Wait()
		close(results)
	}()

	var fetchErr error
	for result := range results {
		if result.err != nil {
			if fetchErr == nil {
				fetchErr = result.err
				cancel()
			}
			continue
		}
		appendPage(result.page)
	}
	if fetchErr != nil {
		return nil, fetchErr
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	return values, nil
}

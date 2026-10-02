package crawler

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"reflect"
	"sort"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func testClient(t *testing.T, handler http.HandlerFunc, parallel int) *Client {
	t.Helper()
	server := httptest.NewServer(handler)
	t.Cleanup(server.Close)
	client, err := New(parallel)
	if err != nil {
		t.Fatal(err)
	}
	client.endpoint = server.URL + "/api/tags"
	client.interval = 0
	client.retryDelay = 0
	return client
}

func pageJSON(total, offset int) string {
	items := make([]item, min(pageSize, max(0, total-offset)))
	for i := range items {
		items[i].Registrable = fmt.Sprintf("site%d.example", offset+i)
	}
	data, _ := json.Marshal(page{Total: &total, Items: &items})
	return string(data)
}

func TestNew(t *testing.T) {
	for _, parallel := range []int{-1, 0} {
		if _, err := New(parallel); err == nil {
			t.Errorf("New(%d) should fail", parallel)
		}
	}
	client, err := New(10)
	if err != nil {
		t.Fatal(err)
	}
	if client.interval != 100*time.Millisecond || client.maxParallel != 10 || client.http.Timeout <= 0 {
		t.Fatalf("incorrect defaults: %+v", client)
	}
}

func TestFetchPagination(t *testing.T) {
	for _, total := range []int{0, 1, 199, 200, 201, 400, 450, 2401} {
		t.Run(strconv.Itoa(total), func(t *testing.T) {
			var mu sync.Mutex
			var offsets []int
			client := testClient(t, func(w http.ResponseWriter, r *http.Request) {
				query := r.URL.Query()
				offset, err := strconv.Atoi(query.Get("offset"))
				if err != nil || offset%pageSize != 0 || query.Get("limit") != "200" ||
					query.Get("status") != "blocked" || r.URL.Path != "/api/tags" ||
					r.Method != http.MethodGet || r.Header.Get("Accept") != "application/json" {
					t.Errorf("unexpected request: %s", r.URL)
				}
				mu.Lock()
				offsets = append(offsets, offset)
				mu.Unlock()
				io.WriteString(w, pageJSON(total, offset))
			}, 10)
			values, err := client.Fetch(context.Background())
			if err != nil {
				t.Fatal(err)
			}
			if len(values) != total {
				t.Fatalf("got %d values, want %d", len(values), total)
			}
			seen := make(map[string]bool)
			for _, value := range values {
				seen[value] = true
			}
			for i := 0; i < total; i++ {
				if !seen[fmt.Sprintf("site%d.example", i)] {
					t.Fatalf("missing item %d", i)
				}
			}
			mu.Lock()
			defer mu.Unlock()
			sort.Ints(offsets)
			expected := []int{0}
			for offset := pageSize; offset < total; offset += pageSize {
				expected = append(expected, offset)
			}
			if !reflect.DeepEqual(offsets, expected) {
				t.Fatalf("offsets %v, want %v", offsets, expected)
			}
		})
	}
}

func TestFetchParallelLimit(t *testing.T) {
	for _, parallel := range []int{1, 3, 10} {
		t.Run(strconv.Itoa(parallel), func(t *testing.T) {
			var mu sync.Mutex
			active, peak := 0, 0
			gate := make(chan struct{})
			var once sync.Once
			client := testClient(t, func(w http.ResponseWriter, r *http.Request) {
				offset, _ := strconv.Atoi(r.URL.Query().Get("offset"))
				if offset != 0 {
					mu.Lock()
					active++
					peak = max(peak, active)
					if active == parallel {
						once.Do(func() { close(gate) })
					}
					mu.Unlock()
					defer func() {
						mu.Lock()
						active--
						mu.Unlock()
					}()
					select {
					case <-gate:
					case <-r.Context().Done():
						return
					}
				}
				io.WriteString(w, pageJSON(2401, offset))
			}, parallel)
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			if _, err := client.Fetch(ctx); err != nil {
				t.Fatal(err)
			}
			mu.Lock()
			defer mu.Unlock()
			if peak != parallel {
				t.Fatalf("peak concurrency = %d, want %d", peak, parallel)
			}
		})
	}
}

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(r *http.Request) (*http.Response, error) {
	return f(r)
}

func TestFetchPacesRequestsAndRetries(t *testing.T) {
	client, _ := New(10)
	client.interval = 20 * time.Millisecond
	client.retryDelay = 0
	var mu sync.Mutex
	var starts []time.Time
	client.http.Transport = roundTripFunc(func(r *http.Request) (*http.Response, error) {
		mu.Lock()
		defer mu.Unlock()
		starts = append(starts, time.Now())
		status := http.StatusOK
		if len(starts) == 2 {
			status = http.StatusTooManyRequests
		}
		offset, _ := strconv.Atoi(r.URL.Query().Get("offset"))
		return &http.Response{
			StatusCode: status,
			Body:       io.NopCloser(strings.NewReader(pageJSON(801, offset))),
			Header:     make(http.Header),
		}, nil
	})
	if _, err := client.Fetch(context.Background()); err != nil {
		t.Fatal(err)
	}
	mu.Lock()
	defer mu.Unlock()
	if len(starts) != 6 {
		t.Fatalf("got %d requests, want 6 (including retry)", len(starts))
	}
	for i := 1; i < len(starts); i++ {
		if delay := starts[i].Sub(starts[i-1]); delay < client.interval-time.Millisecond {
			t.Errorf("request gap %s is shorter than %s", delay, client.interval)
		}
	}
}

func TestFetchRetries(t *testing.T) {
	for _, status := range []int{http.StatusTooManyRequests, http.StatusInternalServerError, http.StatusServiceUnavailable} {
		t.Run(strconv.Itoa(status), func(t *testing.T) {
			var requests atomic.Int32
			client := testClient(t, func(w http.ResponseWriter, r *http.Request) {
				if requests.Add(1) < 3 {
					w.WriteHeader(status)
					return
				}
				io.WriteString(w, pageJSON(1, 0))
			}, 1)
			values, err := client.Fetch(context.Background())
			if err != nil || len(values) != 1 || requests.Load() != 3 {
				t.Fatalf("values=%v err=%v requests=%d", values, err, requests.Load())
			}
		})
	}
}

func TestFetchHTTPFailure(t *testing.T) {
	for _, status := range []int{http.StatusForbidden, http.StatusServiceUnavailable} {
		t.Run(strconv.Itoa(status), func(t *testing.T) {
			var requests atomic.Int32
			client := testClient(t, func(w http.ResponseWriter, r *http.Request) {
				requests.Add(1)
				w.WriteHeader(status)
			}, 10)
			values, err := client.Fetch(context.Background())
			expected := int32(1)
			if status >= 500 {
				expected = 3
			}
			if err == nil || values != nil || requests.Load() != expected {
				t.Fatalf("values=%v err=%v requests=%d", values, err, requests.Load())
			}
		})
	}
}

func TestFetchPageFailureCancelsWorkers(t *testing.T) {
	client := testClient(t, func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Query().Get("offset") {
		case "0":
			io.WriteString(w, pageJSON(1000, 0))
		case "200":
			w.WriteHeader(http.StatusForbidden)
		default:
			<-r.Context().Done()
		}
	}, 3)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	values, err := client.Fetch(ctx)
	if values != nil || err == nil || !strings.Contains(err.Error(), "403") || ctx.Err() != nil {
		t.Fatalf("values=%v err=%v context error=%v", values, err, ctx.Err())
	}
}

func TestFetchCancellation(t *testing.T) {
	client := testClient(t, func(w http.ResponseWriter, r *http.Request) {
		t.Error("request made after cancellation")
	}, 10)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := client.Fetch(ctx); err != context.Canceled {
		t.Fatalf("got %v, want cancellation", err)
	}
}

func TestDecodePageRejectsInvalidResponses(t *testing.T) {
	for _, body := range []string{
		`<html>error</html>`, `{`, `{}`, `null`,
		`{"total":-1,"items":[]}`, `{"total":0}`,
		`{"items":[]}`, `{"total":0,"items":null}`,
		`{"total":"1","items":[]}`, `{"total":1.5,"items":[]}`,
		`{"total":1,"items":[]}`, `{"total":0,"items":[{"registrable":"example.com"}]}`,
		`{"total":1,"items":[{"registrable":123}]}`,
		`{"total":0,"items":[]} {}`, `{"total":0,"items":[]} garbage`,
	} {
		t.Run(body, func(t *testing.T) {
			if _, err := decodePage(strings.NewReader(body), 0); err == nil {
				t.Fatal("expected an error")
			}
		})
	}
}

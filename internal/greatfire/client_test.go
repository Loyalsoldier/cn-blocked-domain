package greatfire

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"net/http"
	"net/http/httptest"
	"slices"
	"strconv"
	"sync/atomic"
	"testing"
	"time"
)

func TestFetchAll(t *testing.T) {
	const total = 450
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		q := r.URL.Query()
		if q.Get("status") != "blocked" || q.Get("limit") != "200" {
			http.Error(w, "bad query", http.StatusBadRequest)
			return
		}
		off, _ := strconv.Atoi(q.Get("offset"))
		p := Page{Total: total}
		for i := off; i < min(off+PageSize, total); i++ {
			p.Items = append(p.Items, Item{Registrable: "d" + strconv.Itoa(i) + ".com"})
		}
		json.NewEncoder(w).Encode(p)
	}))
	defer srv.Close()

	c := NewClient(2)
	c.BaseURL = srv.URL
	c.Delay = time.Millisecond
	got, err := c.FetchAll(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != total || got[0] != "d0.com" || got[total-1] != "d449.com" {
		t.Fatalf("unexpected result: len=%d", len(got))
	}
}

func TestFetchAllMaxPages(t *testing.T) {
	var requests atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests.Add(1)
		off, _ := strconv.Atoi(r.URL.Query().Get("offset"))
		p := Page{Total: 10000}
		for i := off; i < off+PageSize; i++ {
			p.Items = append(p.Items, Item{Registrable: "d" + strconv.Itoa(i) + ".com"})
		}
		json.NewEncoder(w).Encode(p)
	}))
	defer srv.Close()

	c := NewClient(2)
	c.BaseURL = srv.URL
	c.Delay = time.Millisecond
	c.MaxPages = 3
	got, err := c.FetchAll(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 3*PageSize || requests.Load() != 3 {
		t.Fatalf("got %d entries in %d requests, want %d in 3", len(got), requests.Load(), 3*PageSize)
	}
}

func TestFetchAllError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "boom", http.StatusInternalServerError)
	}))
	defer srv.Close()
	c := NewClient(1)
	c.BaseURL = srv.URL
	c.Retries = 0
	if _, err := c.FetchAll(context.Background()); err == nil {
		t.Fatal("expected error")
	}
}

func TestFetchAllInvalidPage(t *testing.T) {
	for _, body := range []string{
		`null`,
		`{}`,
		`{"error":"temporarily unavailable"}`,
		`{"total":-1,"items":[]}`,
		`{"total":1}`,
		`{"total":1,"items":null}`,
		`{"total":1,"items":[]}`,
		`{"total":3,"items":[{"registrable":"example.com"}]}`,
		`{"total":0,"items":[{"registrable":"example.com"}]}`,
	} {
		t.Run(body, func(t *testing.T) {
			for _, badOffset := range []int{0, PageSize} {
				t.Run(strconv.Itoa(badOffset), func(t *testing.T) {
					srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
						if r.URL.Query().Get("offset") == strconv.Itoa(badOffset) {
							fmt.Fprint(w, body)
							return
						}
						json.NewEncoder(w).Encode(Page{Total: PageSize + 1, Items: make([]Item, PageSize)})
					}))
					defer srv.Close()
					c := NewClient(2)
					c.BaseURL = srv.URL
					c.Retries = 0
					got, err := c.FetchAll(context.Background())
					if err == nil || got != nil {
						t.Fatalf("got %v, %v; want no results and an error", got, err)
					}
				})
			}
		})
	}
}

func TestFetchAllEmpty(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, `{"total":0,"items":[]}`)
	}))
	defer srv.Close()
	c := NewClient(1)
	c.BaseURL = srv.URL
	got, err := c.FetchAll(context.Background())
	if err != nil || len(got) != 0 {
		t.Fatalf("got %v, %v; want no entries and no error", got, err)
	}
}

func TestFetchAllLargeLimits(t *testing.T) {
	const total = 3*PageSize + 1
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		off, _ := strconv.Atoi(r.URL.Query().Get("offset"))
		p := Page{Total: total}
		for i := off; i < min(off+PageSize, total); i++ {
			p.Items = append(p.Items, Item{Registrable: strconv.Itoa(i) + ".com"})
		}
		json.NewEncoder(w).Encode(p)
	}))
	defer srv.Close()
	c := NewClient(2)
	c.BaseURL = srv.URL
	c.Delay = 0
	c.MaxPages = math.MaxInt
	got, err := c.FetchAll(context.Background())
	if err != nil || len(got) != total {
		t.Fatalf("got %d entries, %v; want %d entries", len(got), err, total)
	}
	c.Parallel = math.MaxInt
	got, err = c.FetchAll(context.Background())
	if err != nil || len(got) != total {
		t.Fatalf("got %d entries, %v; want %d entries", len(got), err, total)
	}
}

func TestFetchAllRetriesInvalidPage(t *testing.T) {
	var requests atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if requests.Add(1) == 1 {
			fmt.Fprint(w, `{"error":"temporarily unavailable"}`)
			return
		}
		fmt.Fprint(w, `{"total":1,"items":[{"registrable":"example.com"}]}`)
	}))
	defer srv.Close()
	c := NewClient(1)
	c.BaseURL = srv.URL
	c.Retries = 1
	got, err := c.FetchAll(context.Background())
	if err != nil || !slices.Equal(got, []string{"example.com"}) || requests.Load() != 2 {
		t.Fatalf("got %v, %v after %d requests; want example.com after 2 requests", got, err, requests.Load())
	}
}

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(r *http.Request) (*http.Response, error) {
	return f(r)
}

func TestFetchBatchCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	c := NewClient(2)
	c.Delay = time.Hour
	c.Retries = 0
	finished := make(chan struct{})
	c.HTTPClient = &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
		cancel()
		time.Sleep(10 * time.Millisecond)
		close(finished)
		return nil, r.Context().Err()
	})}
	if _, err := c.fetchBatch(ctx, []int{PageSize, 2 * PageSize}); !errors.Is(err, context.Canceled) {
		t.Fatalf("got %v; want context.Canceled", err)
	}
	select {
	case <-finished:
	default:
		t.Error("fetchBatch returned before its in-flight request finished")
		<-finished
	}
}

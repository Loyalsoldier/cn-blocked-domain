package greatfire

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strconv"
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

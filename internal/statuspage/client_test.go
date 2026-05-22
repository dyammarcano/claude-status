package statuspage

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestFetch_Success(t *testing.T) {
	body := `{"page":{"updated_at":"2026-05-22T00:00:00Z"},"status":{"indicator":"minor","description":"Partial outage"}}`
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(body))
	}))
	defer srv.Close()

	c := NewClient(srv.URL, "test-agent", 2*time.Second)
	got, err := c.Fetch(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got.Indicator != "minor" || got.Description != "Partial outage" {
		t.Fatalf("unexpected status: %+v", got)
	}
}

func TestFetch_Non2xx(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		http.Error(w, "boom", http.StatusInternalServerError)
	}))
	defer srv.Close()

	c := NewClient(srv.URL, "test-agent", 2*time.Second)
	if _, err := c.Fetch(context.Background()); err == nil {
		t.Fatal("expected error, got nil")
	}
}

func TestFetch_BadJSON(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte("not json"))
	}))
	defer srv.Close()

	c := NewClient(srv.URL, "test-agent", 2*time.Second)
	if _, err := c.Fetch(context.Background()); err == nil {
		t.Fatal("expected error, got nil")
	}
}

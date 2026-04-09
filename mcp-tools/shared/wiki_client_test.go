package shared

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestGetPage(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/wiki/customers/john-smith" {
			t.Errorf("unexpected path: %s", r.URL.Path)
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", "text/markdown")
		w.Write([]byte("# John Smith\nCredit Score: 782"))
	}))
	defer srv.Close()

	client := NewWikiClient(srv.URL)
	content, err := client.GetPage("customers/john-smith")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if content != "# John Smith\nCredit Score: 782" {
		t.Errorf("unexpected content: %s", content)
	}
}

func TestSearch(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/search" {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		if r.URL.Query().Get("q") != "john" {
			t.Errorf("unexpected query: %s", r.URL.Query().Get("q"))
		}
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`[{"path":"customers/john-smith","title":"John Smith","snippet":"...","score":5.2}]`))
	}))
	defer srv.Close()

	client := NewWikiClient(srv.URL)
	results, err := client.Search("john")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(results) != 1 || results[0].Path != "customers/john-smith" {
		t.Errorf("unexpected results: %+v", results)
	}
}

func TestListPages(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`[{"name":"john-smith","title":"John Smith","path":"customers/john-smith"}]`))
	}))
	defer srv.Close()

	client := NewWikiClient(srv.URL)
	pages, err := client.ListPages("customers")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(pages) != 1 {
		t.Errorf("expected 1 page, got %d", len(pages))
	}
}

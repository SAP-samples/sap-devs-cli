package update

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestCheckLatest_NewerAvailable(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(map[string]string{"tag_name": "v9.9.9"})
	}))
	defer srv.Close()
	apiBase = srv.URL
	t.Cleanup(func() { apiBase = "" })

	rel, newer, err := CheckLatest("https://example.com/owner/repo", "1.0.0", "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !newer {
		t.Fatal("expected newer=true")
	}
	if rel.Version != "9.9.9" {
		t.Fatalf("expected Version=9.9.9, got %s", rel.Version)
	}
	if rel.TagName != "v9.9.9" {
		t.Fatalf("expected TagName=v9.9.9, got %s", rel.TagName)
	}
}

func TestCheckLatest_AlreadyLatest(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(map[string]string{"tag_name": "v1.0.0"})
	}))
	defer srv.Close()
	apiBase = srv.URL
	t.Cleanup(func() { apiBase = "" })

	_, newer, err := CheckLatest("https://example.com/owner/repo", "1.0.0", "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if newer {
		t.Fatal("expected newer=false")
	}
}

func TestCheckLatest_DevBuildSameBase(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(map[string]string{"tag_name": "v0.0.8"})
	}))
	defer srv.Close()
	apiBase = srv.URL
	t.Cleanup(func() { apiBase = "" })

	_, newer, err := CheckLatest("https://example.com/owner/repo", "v0.0.8-2-gb16d6f0-dirty", "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if newer {
		t.Fatal("expected newer=false for dev build based on same version")
	}
}

func TestCheckLatest_DevBuildOlderBase(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(map[string]string{"tag_name": "v0.0.9"})
	}))
	defer srv.Close()
	apiBase = srv.URL
	t.Cleanup(func() { apiBase = "" })

	_, newer, err := CheckLatest("https://example.com/owner/repo", "v0.0.8-5-gabcdef0", "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !newer {
		t.Fatal("expected newer=true when release is ahead of dev build base")
	}
}

func TestCheckLatest_NetworkError(t *testing.T) {
	apiBase = "http://127.0.0.1:1" // nothing listening
	t.Cleanup(func() { apiBase = "" })

	_, _, err := CheckLatest("https://example.com/owner/repo", "1.0.0", "")
	if err == nil {
		t.Fatal("expected error on network failure")
	}
}

func TestCheckLatest_MalformedJSON(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("not json"))
	}))
	defer srv.Close()
	apiBase = srv.URL
	t.Cleanup(func() { apiBase = "" })

	_, _, err := CheckLatest("https://example.com/owner/repo", "1.0.0", "")
	if err == nil {
		t.Fatal("expected error on malformed JSON")
	}
}

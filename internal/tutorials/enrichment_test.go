package tutorials_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.tools.sap/developer-relations/sap-devs-cli/internal/tutorials"
)

func TestEnrich_403_ReturnsOriginal(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusForbidden)
	}))
	defer ts.Close()

	index := []tutorials.TutorialMeta{{Slug: "test", Title: "Test"}}
	got := tutorials.EnrichWithURL(index, "test-agent", ts.URL)
	assert.Len(t, got, 1)
	assert.Equal(t, "test", got[0].Slug)
}

func TestEnrich_200_ReturnsOriginal(t *testing.T) {
	result := map[string]any{
		"result": []map[string]any{
			{"publicUrl": "/tutorials/test.html", "featured": true},
		},
	}
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(result)
	}))
	defer ts.Close()

	index := []tutorials.TutorialMeta{{Slug: "test", Title: "Test"}}
	got := tutorials.EnrichWithURL(index, "test-agent", ts.URL)
	assert.Len(t, got, 1)
}

func TestEnrich_MalformedJSON_ReturnsOriginal(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("not json"))
	}))
	defer ts.Close()

	index := []tutorials.TutorialMeta{{Slug: "test", Title: "Test"}}
	got := tutorials.EnrichWithURL(index, "test-agent", ts.URL)
	assert.Len(t, got, 1)
}

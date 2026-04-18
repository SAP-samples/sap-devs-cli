package tutorials_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.tools.sap/developer-relations/sap-devs-cli/internal/tutorials"
)

func TestFetchRepoList(t *testing.T) {
	repos := []map[string]string{
		{"name": "Tutorials", "urlBase": "https://github.com/sap-tutorials/Tutorials"},
		{"name": "abap-core-development", "urlBase": "https://github.com/sap-tutorials/abap-core-development"},
	}
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(repos)
	}))
	defer ts.Close()

	client := tutorials.NewClient(tutorials.ClientConfig{RepoListURL: ts.URL})
	got, err := client.FetchRepoList()
	require.NoError(t, err)
	require.Len(t, got, 2)
	assert.Equal(t, "Tutorials", got[0])
}

func TestFetchRepoTree(t *testing.T) {
	treeResp := map[string]any{
		"sha": "abc123",
		"tree": []map[string]string{
			{"path": "tutorials/cap-getting-started/cap-getting-started.md", "type": "blob"},
			{"path": "tutorials/abap-rap/abap-rap.md", "type": "blob"},
			{"path": "README.md", "type": "blob"},
		},
	}
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(treeResp)
	}))
	defer ts.Close()

	client := tutorials.NewClient(tutorials.ClientConfig{APIBaseURL: ts.URL})
	slugs, sha, err := client.FetchRepoTree("Tutorials", "master")
	require.NoError(t, err)
	assert.Equal(t, "abc123", sha)
	require.Len(t, slugs, 2)
	assert.Contains(t, slugs, "cap-getting-started")
	assert.Contains(t, slugs, "abap-rap")
}

func TestFetchDefaultBranch(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(map[string]string{"default_branch": "master"})
	}))
	defer ts.Close()

	client := tutorials.NewClient(tutorials.ClientConfig{APIBaseURL: ts.URL})
	branch, err := client.FetchDefaultBranch("Tutorials")
	require.NoError(t, err)
	assert.Equal(t, "master", branch)
}

func TestFetchRawMarkdown(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("---\nparser: v2\ntime: 20\ntags: [tutorial>beginner]\nprimary_tag: x\n---\n\n# Test\n\n### Step 1\nContent\n"))
	}))
	defer ts.Close()

	client := tutorials.NewClient(tutorials.ClientConfig{RawBaseURL: ts.URL})
	md, err := client.FetchRawMarkdown("Tutorials", "main", "test-slug")
	require.NoError(t, err)
	assert.Contains(t, md, "# Test")
}

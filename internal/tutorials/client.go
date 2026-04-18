package tutorials

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

const (
	defaultRepoListURL = "https://raw.githubusercontent.com/sap-tutorials/Tutorials/master/config/repository-groups.json"
	defaultAPIBaseURL  = "https://api.github.com"
	defaultRawBaseURL  = "https://raw.githubusercontent.com"
)

// ClientConfig allows overriding base URLs for testing.
type ClientConfig struct {
	RepoListURL string
	APIBaseURL  string
	RawBaseURL  string
	Token       string
	UserAgent   string
}

// Client handles GitHub API interactions for tutorials.
type Client struct {
	http   *http.Client
	config ClientConfig
}

// NewClient creates a new tutorial GitHub client.
func NewClient(cfg ClientConfig) *Client {
	if cfg.RepoListURL == "" {
		cfg.RepoListURL = defaultRepoListURL
	}
	if cfg.APIBaseURL == "" {
		cfg.APIBaseURL = defaultAPIBaseURL
	}
	if cfg.RawBaseURL == "" {
		cfg.RawBaseURL = defaultRawBaseURL
	}
	if cfg.UserAgent == "" {
		cfg.UserAgent = "sap-devs-cli"
	}
	return &Client{
		http:   &http.Client{Timeout: 30 * time.Second},
		config: cfg,
	}
}

type repoGroupEntry struct {
	Name string `json:"name"`
}

// FetchRepoList fetches the list of tutorial repo names.
func (c *Client) FetchRepoList() ([]string, error) {
	body, err := c.get(c.config.RepoListURL)
	if err != nil {
		return nil, fmt.Errorf("fetch repo list: %w", err)
	}
	var entries []repoGroupEntry
	if err := json.Unmarshal(body, &entries); err != nil {
		return nil, fmt.Errorf("parse repo list: %w", err)
	}
	var names []string
	for _, e := range entries {
		if e.Name != "" {
			names = append(names, e.Name)
		}
	}
	return names, nil
}

// FetchDefaultBranch returns the default branch for a repo.
func (c *Client) FetchDefaultBranch(repo string) (string, error) {
	url := fmt.Sprintf("%s/repos/sap-tutorials/%s", c.config.APIBaseURL, repo)
	body, err := c.get(url)
	if err != nil {
		return "", fmt.Errorf("fetch repo info %s: %w", repo, err)
	}
	var info struct {
		DefaultBranch string `json:"default_branch"`
	}
	if err := json.Unmarshal(body, &info); err != nil {
		return "", err
	}
	return info.DefaultBranch, nil
}

// FetchRepoTree fetches the tree for a repo and returns tutorial slugs + tree SHA.
func (c *Client) FetchRepoTree(repo, branch string) (slugs []string, sha string, err error) {
	url := fmt.Sprintf("%s/repos/sap-tutorials/%s/git/trees/%s?recursive=1", c.config.APIBaseURL, repo, branch)
	body, err := c.get(url)
	if err != nil {
		return nil, "", fmt.Errorf("fetch tree %s: %w", repo, err)
	}
	var tree struct {
		SHA  string `json:"sha"`
		Tree []struct {
			Path string `json:"path"`
			Type string `json:"type"`
		} `json:"tree"`
	}
	if err := json.Unmarshal(body, &tree); err != nil {
		return nil, "", err
	}

	seen := make(map[string]bool)
	for _, entry := range tree.Tree {
		if !strings.HasPrefix(entry.Path, "tutorials/") {
			continue
		}
		parts := strings.Split(entry.Path, "/")
		if len(parts) >= 2 && parts[1] != "" {
			slug := parts[1]
			if !seen[slug] {
				seen[slug] = true
				slugs = append(slugs, slug)
			}
		}
	}
	return slugs, tree.SHA, nil
}

// FetchRawMarkdown fetches the raw markdown content for a tutorial.
func (c *Client) FetchRawMarkdown(repo, branch, slug string) (string, error) {
	url := fmt.Sprintf("%s/sap-tutorials/%s/%s/tutorials/%s/%s.md", c.config.RawBaseURL, repo, branch, slug, slug)
	body, err := c.get(url)
	if err != nil {
		return "", fmt.Errorf("fetch markdown %s/%s: %w", repo, slug, err)
	}
	return string(body), nil
}

func (c *Client) get(url string) ([]byte, error) {
	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", c.config.UserAgent)
	if c.config.Token != "" {
		req.Header.Set("Authorization", "token "+c.config.Token)
	}
	resp, err := c.http.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("HTTP %d for %s", resp.StatusCode, url)
	}
	return io.ReadAll(resp.Body)
}

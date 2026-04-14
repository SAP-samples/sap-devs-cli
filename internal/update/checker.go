package update

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"strings"
)

// apiBase overrides the GitHub API base URL in tests.
// When empty, it is derived from the repoURL (github enterprise pattern).
var apiBase string

// Release holds the relevant fields from a GitHub release.
type Release struct {
	Version string // e.g. "1.2.0" (no leading "v")
	TagName string // e.g. "v1.2.0"
}

// CheckLatest fetches the latest GitHub release and returns it along with
// whether it is newer than currentVersion.
// Returns a real error on failure — callers decide whether to surface or swallow it.
func CheckLatest(repoURL, currentVersion string) (*Release, bool, error) {
	apiURL, err := buildAPIURL(repoURL)
	if err != nil {
		return nil, false, fmt.Errorf("invalid repo URL: %w", err)
	}

	req, err := http.NewRequest("GET", apiURL, nil)
	if err != nil {
		return nil, false, err
	}
	req.Header.Set("Accept", "application/vnd.github+json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, false, fmt.Errorf("could not reach GitHub: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, false, fmt.Errorf("GitHub API returned HTTP %d", resp.StatusCode)
	}

	var result struct {
		TagName string `json:"tag_name"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, false, fmt.Errorf("could not parse release response: %w", err)
	}
	if result.TagName == "" {
		return nil, false, fmt.Errorf("release response missing tag_name")
	}

	rel := &Release{
		TagName: result.TagName,
		Version: strings.TrimPrefix(result.TagName, "v"),
	}
	newer := compareVersions(rel.Version, strings.TrimPrefix(currentVersion, "v")) > 0
	return rel, newer, nil
}

// buildAPIURL constructs the releases/latest API URL from a repo URL.
// If apiBase is set (tests), uses that as the base directly.
func buildAPIURL(repoURL string) (string, error) {
	if apiBase != "" {
		// tests: apiBase is the full server URL; just append a known path
		u, err := url.Parse(repoURL)
		if err != nil {
			return "", err
		}
		parts := strings.Split(strings.Trim(u.Path, "/"), "/")
		if len(parts) < 2 {
			return "", fmt.Errorf("expected owner/repo in path, got %q", u.Path)
		}
		owner, repo := parts[len(parts)-2], parts[len(parts)-1]
		return apiBase + "/repos/" + owner + "/" + repo + "/releases/latest", nil
	}

	u, err := url.Parse(repoURL)
	if err != nil {
		return "", err
	}
	parts := strings.Split(strings.Trim(u.Path, "/"), "/")
	if len(parts) < 2 {
		return "", fmt.Errorf("expected owner/repo in path, got %q", u.Path)
	}
	owner, repo := parts[len(parts)-2], parts[len(parts)-1]
	// GitHub Enterprise: <host>/api/v3/repos/<owner>/<repo>/releases/latest
	base := u.Scheme + "://" + u.Host + "/api/v3"
	return base + "/repos/" + owner + "/" + repo + "/releases/latest", nil
}

// compareVersions compares two "major.minor.patch" version strings (no "v" prefix).
// Returns >0 if a > b, 0 if equal, <0 if a < b.
// Uses integer comparison field by field. Missing fields treated as 0.
func compareVersions(a, b string) int {
	aParts := strings.Split(a, ".")
	bParts := strings.Split(b, ".")
	n := len(aParts)
	if len(bParts) > n {
		n = len(bParts)
	}
	for i := 0; i < n; i++ {
		av, bv := 0, 0
		if i < len(aParts) {
			av, _ = strconv.Atoi(aParts[i])
		}
		if i < len(bParts) {
			bv, _ = strconv.Atoi(bParts[i])
		}
		if av != bv {
			return av - bv
		}
	}
	return 0
}

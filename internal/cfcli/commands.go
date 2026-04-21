package cfcli

import (
	"context"
	"fmt"
	"regexp"
	"strings"
)

var reCFRegion = regexp.MustCompile(`api\.cf\.([a-z0-9-]+)\.hana\.ondemand\.com`)

type TargetInfo struct {
	Org      string `json:"org"`
	Space    string `json:"space"`
	API      string `json:"api"`
	Region   string `json:"region"`
	LoggedIn bool   `json:"logged_in"`
}

func (c *Client) Target(ctx context.Context) (TargetInfo, error) {
	cfg := c.readConfig()
	if cfg != nil && cfg.OrganizationFields.Name != "" {
		region := ""
		if m := reCFRegion.FindStringSubmatch(cfg.Target); len(m) >= 2 {
			region = m[1]
		}
		return TargetInfo{
			Org:      cfg.OrganizationFields.Name,
			Space:    cfg.SpaceFields.Name,
			API:      cfg.Target,
			Region:   region,
			LoggedIn: cfg.AccessToken != "",
		}, nil
	}

	childCtx, cancel := context.WithTimeout(ctx, c.timeout)
	defer cancel()

	out, err := c.runWithContext(childCtx, "cf target")
	if err != nil {
		if notInstalled := checkNotInstalled(err); notInstalled != nil {
			return TargetInfo{}, notInstalled
		}
		if authErr := checkAuthError(out); authErr != nil {
			return TargetInfo{}, authErr
		}
		return TargetInfo{}, fmt.Errorf("cf target failed: %s", out)
	}
	if authErr := checkAuthError(out); authErr != nil {
		return TargetInfo{}, authErr
	}

	info := parseCFTarget(out)
	return info, nil
}

var _ = strings.TrimSpace // suppress unused import until more commands are added

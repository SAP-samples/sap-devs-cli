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

func (c *Client) Apps(ctx context.Context) ([]App, error) {
	childCtx, cancel := context.WithTimeout(ctx, c.timeout)
	defer cancel()

	out, err := c.runWithContext(childCtx, "cf apps")
	if err != nil {
		if notInstalled := checkNotInstalled(err); notInstalled != nil {
			return nil, notInstalled
		}
		if authErr := checkAuthError(out); authErr != nil {
			return nil, authErr
		}
		return nil, fmt.Errorf("cf apps failed: %s", out)
	}
	if authErr := checkAuthError(out); authErr != nil {
		return nil, authErr
	}
	return parseCFApps(out), nil
}

func (c *Client) Services(ctx context.Context) ([]Service, error) {
	childCtx, cancel := context.WithTimeout(ctx, c.timeout)
	defer cancel()

	out, err := c.runWithContext(childCtx, "cf services")
	if err != nil {
		if notInstalled := checkNotInstalled(err); notInstalled != nil {
			return nil, notInstalled
		}
		if authErr := checkAuthError(out); authErr != nil {
			return nil, authErr
		}
		return nil, fmt.Errorf("cf services failed: %s", out)
	}
	if authErr := checkAuthError(out); authErr != nil {
		return nil, authErr
	}
	return parseCFServices(out), nil
}

type AppEnv struct {
	SystemProvided any `json:"system_provided,omitempty"`
	UserProvided   any `json:"user_provided,omitempty"`
	Running        any `json:"running_env,omitempty"`
	Staging        any `json:"staging_env,omitempty"`
}

func (c *Client) Env(ctx context.Context, appName string) (AppEnv, error) {
	if appName == "" || strings.ContainsAny(appName, " \t\n\r;|&$`'\"\\") {
		return AppEnv{}, fmt.Errorf("invalid app name: %q", appName)
	}

	childCtx, cancel := context.WithTimeout(ctx, c.timeout)
	defer cancel()

	out, err := c.runWithContext(childCtx, "cf env "+appName)
	if err != nil {
		if notInstalled := checkNotInstalled(err); notInstalled != nil {
			return AppEnv{}, notInstalled
		}
		if authErr := checkAuthError(out); authErr != nil {
			return AppEnv{}, authErr
		}
		return AppEnv{}, fmt.Errorf("cf env failed: %s", out)
	}
	if authErr := checkAuthError(out); authErr != nil {
		return AppEnv{}, authErr
	}
	return parseCFEnv(out), nil
}

func (c *Client) Routes(ctx context.Context) ([]Route, error) {
	childCtx, cancel := context.WithTimeout(ctx, c.timeout)
	defer cancel()

	out, err := c.runWithContext(childCtx, "cf routes")
	if err != nil {
		if notInstalled := checkNotInstalled(err); notInstalled != nil {
			return nil, notInstalled
		}
		if authErr := checkAuthError(out); authErr != nil {
			return nil, authErr
		}
		return nil, fmt.Errorf("cf routes failed: %s", out)
	}
	if authErr := checkAuthError(out); authErr != nil {
		return nil, authErr
	}
	return parseCFRoutes(out), nil
}

func (c *Client) Domains(ctx context.Context) ([]Domain, error) {
	childCtx, cancel := context.WithTimeout(ctx, c.timeout)
	defer cancel()

	out, err := c.runWithContext(childCtx, "cf domains")
	if err != nil {
		if notInstalled := checkNotInstalled(err); notInstalled != nil {
			return nil, notInstalled
		}
		if authErr := checkAuthError(out); authErr != nil {
			return nil, authErr
		}
		return nil, fmt.Errorf("cf domains failed: %s", out)
	}
	if authErr := checkAuthError(out); authErr != nil {
		return nil, authErr
	}
	return parseCFDomains(out), nil
}

func (c *Client) Buildpacks(ctx context.Context) ([]Buildpack, error) {
	childCtx, cancel := context.WithTimeout(ctx, c.timeout)
	defer cancel()

	out, err := c.runWithContext(childCtx, "cf buildpacks")
	if err != nil {
		if notInstalled := checkNotInstalled(err); notInstalled != nil {
			return nil, notInstalled
		}
		if authErr := checkAuthError(out); authErr != nil {
			return nil, authErr
		}
		return nil, fmt.Errorf("cf buildpacks failed: %s", out)
	}
	if authErr := checkAuthError(out); authErr != nil {
		return nil, authErr
	}
	return parseCFBuildpacks(out), nil
}

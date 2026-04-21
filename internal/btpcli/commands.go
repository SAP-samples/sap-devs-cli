package btpcli

import (
	"context"
	"encoding/json"
	"fmt"
	"regexp"
	"strings"
)

var reBTPRegion = regexp.MustCompile(`^([a-z]{2}\d{2})`)

type TargetInfo struct {
	Subaccount    string `json:"subaccount"`
	GlobalAccount string `json:"global_account"`
	Region        string `json:"region"`
	Trial         bool   `json:"trial"`
	LoggedIn      bool   `json:"logged_in"`
}

func (c *Client) Target(ctx context.Context) (TargetInfo, error) {
	cfg := c.readConfig()
	if cfg != nil && cfg.TargetHierarchy.SubaccountSubdomain != "" {
		subdomain := cfg.TargetHierarchy.SubaccountSubdomain
		region := ""
		if m := reBTPRegion.FindStringSubmatch(subdomain); len(m) >= 2 {
			region = m[1]
		}
		return TargetInfo{
			Subaccount:    subdomain,
			GlobalAccount: cfg.TargetHierarchy.GlobalAccountSubdomain,
			Region:        region,
			Trial:         strings.Contains(strings.ToLower(subdomain), "trial"),
			LoggedIn:      true,
		}, nil
	}

	childCtx, cancel := context.WithTimeout(ctx, c.timeout)
	defer cancel()

	out, err := c.runWithContext(childCtx, "btp --format json target")
	if err != nil {
		if notInstalled := checkNotInstalled(err); notInstalled != nil {
			return TargetInfo{}, notInstalled
		}
		if authErr := checkAuthError(out); authErr != nil {
			return TargetInfo{}, authErr
		}
		return TargetInfo{}, fmt.Errorf("btp target failed: %s", out)
	}
	if authErr := checkAuthError(out); authErr != nil {
		return TargetInfo{}, authErr
	}

	var result struct {
		SubAccount struct {
			Subdomain string `json:"subdomain"`
		} `json:"subAccount"`
		GlobalAccount struct {
			Subdomain string `json:"subdomain"`
		} `json:"globalAccount"`
	}
	if json.Unmarshal([]byte(out), &result) != nil {
		return TargetInfo{}, fmt.Errorf("failed to parse btp target output")
	}

	subdomain := result.SubAccount.Subdomain
	region := ""
	if m := reBTPRegion.FindStringSubmatch(subdomain); len(m) >= 2 {
		region = m[1]
	}
	return TargetInfo{
		Subaccount:    subdomain,
		GlobalAccount: result.GlobalAccount.Subdomain,
		Region:        region,
		Trial:         strings.Contains(strings.ToLower(subdomain), "trial"),
		LoggedIn:      subdomain != "",
	}, nil
}

type Subaccount struct {
	Name      string `json:"name"`
	Subdomain string `json:"subdomain"`
	Region    string `json:"region"`
	State     string `json:"state"`
	Parent    string `json:"parent"`
}

func (c *Client) Subaccounts(ctx context.Context) ([]Subaccount, error) {
	childCtx, cancel := context.WithTimeout(ctx, c.timeout)
	defer cancel()

	out, err := c.runWithContext(childCtx, "btp --format json list accounts/subaccount")
	if err != nil {
		if notInstalled := checkNotInstalled(err); notInstalled != nil {
			return nil, notInstalled
		}
		if authErr := checkAuthError(out); authErr != nil {
			return nil, authErr
		}
		return nil, fmt.Errorf("btp list subaccounts failed: %s", out)
	}
	if authErr := checkAuthError(out); authErr != nil {
		return nil, authErr
	}

	var raw struct {
		Value []struct {
			DisplayName       string `json:"displayName"`
			Subdomain         string `json:"subdomain"`
			Region            string `json:"region"`
			State             string `json:"state"`
			ParentDisplayName string `json:"parentDisplayName"`
		} `json:"value"`
	}
	if err := json.Unmarshal([]byte(out), &raw); err != nil {
		return nil, fmt.Errorf("failed to parse btp subaccounts output: %w", err)
	}

	subs := make([]Subaccount, 0, len(raw.Value))
	for _, v := range raw.Value {
		subs = append(subs, Subaccount{
			Name:      v.DisplayName,
			Subdomain: v.Subdomain,
			Region:    v.Region,
			State:     v.State,
			Parent:    v.ParentDisplayName,
		})
	}
	return subs, nil
}

type ServiceInstance struct {
	Name    string `json:"name"`
	Service string `json:"service"`
	Plan    string `json:"plan"`
	Status  string `json:"status"`
	Created string `json:"created"`
}

func (c *Client) ServiceInstances(ctx context.Context) ([]ServiceInstance, error) {
	childCtx, cancel := context.WithTimeout(ctx, c.timeout)
	defer cancel()

	out, err := c.runWithContext(childCtx, "btp --format json list services/instance")
	if err != nil {
		if notInstalled := checkNotInstalled(err); notInstalled != nil {
			return nil, notInstalled
		}
		if authErr := checkAuthError(out); authErr != nil {
			return nil, authErr
		}
		return nil, fmt.Errorf("btp list service instances failed: %s", out)
	}
	if authErr := checkAuthError(out); authErr != nil {
		return nil, authErr
	}

	var raw []struct {
		Name            string `json:"name"`
		ServicePlanName string `json:"service_plan_name"`
		ServiceName     string `json:"service_name"`
		Ready           bool   `json:"ready"`
		CreatedAt       string `json:"created_at"`
	}
	if err := json.Unmarshal([]byte(out), &raw); err != nil {
		return nil, fmt.Errorf("failed to parse btp service instances output: %w", err)
	}

	instances := make([]ServiceInstance, 0, len(raw))
	for _, v := range raw {
		status := "not ready"
		if v.Ready {
			status = "ready"
		}
		instances = append(instances, ServiceInstance{
			Name:    v.Name,
			Service: v.ServiceName,
			Plan:    v.ServicePlanName,
			Status:  status,
			Created: v.CreatedAt,
		})
	}
	return instances, nil
}

type RoleCollection struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	RoleCount   int    `json:"role_count"`
}

func (c *Client) RoleCollections(ctx context.Context) ([]RoleCollection, error) {
	childCtx, cancel := context.WithTimeout(ctx, c.timeout)
	defer cancel()

	out, err := c.runWithContext(childCtx, "btp --format json list security/role-collection")
	if err != nil {
		if notInstalled := checkNotInstalled(err); notInstalled != nil {
			return nil, notInstalled
		}
		if authErr := checkAuthError(out); authErr != nil {
			return nil, authErr
		}
		return nil, fmt.Errorf("btp list role collections failed: %s", out)
	}
	if authErr := checkAuthError(out); authErr != nil {
		return nil, authErr
	}

	var raw []struct {
		Name        string `json:"name"`
		Description string `json:"description"`
		RoleRefs    []any  `json:"roleReferences"`
	}
	if err := json.Unmarshal([]byte(out), &raw); err != nil {
		return nil, fmt.Errorf("failed to parse btp role collections output: %w", err)
	}

	rcs := make([]RoleCollection, 0, len(raw))
	for _, v := range raw {
		rcs = append(rcs, RoleCollection{
			Name:        v.Name,
			Description: v.Description,
			RoleCount:   len(v.RoleRefs),
		})
	}
	return rcs, nil
}

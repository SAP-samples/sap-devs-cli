package mcpserver

import (
	"context"
	"encoding/json"
	"fmt"
	"os"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
	"github.com/SAP-samples/sap-devs-cli/internal/cfcli"
)

func registerCFTools(s *server.MCPServer, deps Deps) {
	s.AddTool(
		mcp.NewTool("cf_target",
			mcp.WithDescription("Get current CF target (org, space, API endpoint, region, login status). Use to check where the user's CF CLI is pointed before running other cf_ tools."),
		),
		cfTargetHandler(deps),
	)

	s.AddTool(
		mcp.NewTool("cf_apps",
			mcp.WithDescription("List deployed Cloud Foundry apps with state, instances, memory, and routes."),
			mcp.WithNumber("limit",
				mcp.Description("Maximum number of results to return (default 20, max 100)"),
			),
		),
		cfAppsHandler(deps),
	)

	s.AddTool(
		mcp.NewTool("cf_services",
			mcp.WithDescription("List Cloud Foundry service instances with plan, bound apps, and status."),
			mcp.WithNumber("limit",
				mcp.Description("Maximum number of results to return (default 20, max 100)"),
			),
		),
		cfServicesHandler(deps),
	)

	s.AddTool(
		mcp.NewTool("cf_env",
			mcp.WithDescription("Get environment variables for a Cloud Foundry app (credentials redacted). Shows system-provided services, user-provided variables, and running/staging env groups."),
			mcp.WithString("app",
				mcp.Required(),
				mcp.Description("Name of the Cloud Foundry application"),
			),
		),
		cfEnvHandler(deps),
	)

	s.AddTool(
		mcp.NewTool("cf_routes",
			mcp.WithDescription("List Cloud Foundry routes with domain, host, path, and bound apps."),
			mcp.WithNumber("limit",
				mcp.Description("Maximum number of results to return (default 20, max 100)"),
			),
		),
		cfRoutesHandler(deps),
	)

	s.AddTool(
		mcp.NewTool("cf_domains",
			mcp.WithDescription("List Cloud Foundry domains with type (shared/private) and status."),
			mcp.WithNumber("limit",
				mcp.Description("Maximum number of results to return (default 20, max 100)"),
			),
		),
		cfDomainsHandler(deps),
	)

	s.AddTool(
		mcp.NewTool("cf_buildpacks",
			mcp.WithDescription("List Cloud Foundry buildpacks with position, enabled status, and filename."),
			mcp.WithNumber("limit",
				mcp.Description("Maximum number of results to return (default 20, max 100)"),
			),
		),
		cfBuildpacksHandler(deps),
	)
}

func handleCFError(err error, deps Deps) *mcp.CallToolResult {
	switch e := err.(type) {
	case *cfcli.AuthError:
		return cfAuthErrorResult(e, deps)
	case *cfcli.NotInstalledError:
		return cfNotInstalledResult(deps)
	default:
		return mcp.NewToolResultError(err.Error())
	}
}

func cfAuthErrorResult(err *cfcli.AuthError, deps Deps) *mcp.CallToolResult {
	fix := "Run: cf login"
	if deps.CFConfigPath != "" {
		data, readErr := os.ReadFile(deps.CFConfigPath)
		if readErr == nil {
			var cfg struct {
				Target string `json:"Target"`
			}
			if json.Unmarshal(data, &cfg) == nil && cfg.Target != "" {
				fix = fmt.Sprintf("Run: cf login -a %s", cfg.Target)
			}
		}
	}
	result := map[string]string{
		"error":   "not_authenticated",
		"cli":     "cf",
		"message": err.Message,
		"fix":     fix,
		"hint":    "The cf CLI requires an active login session. After logging in, retry the command.",
	}
	b, _ := json.Marshal(result)
	return mcp.NewToolResultText(string(b))
}

func cfNotInstalledResult(deps Deps) *mcp.CallToolResult {
	install := ""
	for _, p := range deps.Packs {
		for _, t := range p.Tools {
			if t.ID == "cf-cli" {
				install = installForCurrentOS(t.Install)
				break
			}
		}
		if install != "" {
			break
		}
	}
	result := map[string]string{
		"error":   "cli_not_installed",
		"cli":     "cf",
		"message": "Cloud Foundry CLI is not installed.",
		"fix":     fmt.Sprintf("Install: %s", install),
		"hint":    "The cf CLI is required for Cloud Foundry operations. Install it and run 'cf login' to authenticate.",
	}
	b, _ := json.Marshal(result)
	return mcp.NewToolResultText(string(b))
}

func cfTargetHandler(deps Deps) server.ToolHandlerFunc {
	return func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		if deps.CFClient == nil {
			return cfNotInstalledResult(deps), nil
		}
		info, err := deps.CFClient.Target(ctx)
		if err != nil {
			return handleCFError(err, deps), nil
		}
		b, _ := json.Marshal(info)
		return mcp.NewToolResultText(string(b)), nil
	}
}

func cfAppsHandler(deps Deps) server.ToolHandlerFunc {
	return func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		if deps.CFClient == nil {
			return cfNotInstalledResult(deps), nil
		}
		apps, err := deps.CFClient.Apps(ctx)
		if err != nil {
			return handleCFError(err, deps), nil
		}
		limit := clampLimit(req.GetInt("limit", 20), 20, 100)
		total := len(apps)
		if limit < total {
			apps = apps[:limit]
		}
		return wrapResults(apps, total, len(apps), "apps", ""), nil
	}
}

func cfServicesHandler(deps Deps) server.ToolHandlerFunc {
	return func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		if deps.CFClient == nil {
			return cfNotInstalledResult(deps), nil
		}
		services, err := deps.CFClient.Services(ctx)
		if err != nil {
			return handleCFError(err, deps), nil
		}
		limit := clampLimit(req.GetInt("limit", 20), 20, 100)
		total := len(services)
		if limit < total {
			services = services[:limit]
		}
		return wrapResults(services, total, len(services), "services", ""), nil
	}
}

func cfEnvHandler(deps Deps) server.ToolHandlerFunc {
	return func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		if deps.CFClient == nil {
			return cfNotInstalledResult(deps), nil
		}
		appName, err := req.RequireString("app")
		if err != nil {
			return mcp.NewToolResultError("app parameter is required"), nil
		}
		env, err := deps.CFClient.Env(ctx, appName)
		if err != nil {
			return handleCFError(err, deps), nil
		}
		b, _ := json.Marshal(env)
		return mcp.NewToolResultText(string(b)), nil
	}
}

func cfRoutesHandler(deps Deps) server.ToolHandlerFunc {
	return func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		if deps.CFClient == nil {
			return cfNotInstalledResult(deps), nil
		}
		routes, err := deps.CFClient.Routes(ctx)
		if err != nil {
			return handleCFError(err, deps), nil
		}
		limit := clampLimit(req.GetInt("limit", 20), 20, 100)
		total := len(routes)
		if limit < total {
			routes = routes[:limit]
		}
		return wrapResults(routes, total, len(routes), "routes", ""), nil
	}
}

func cfDomainsHandler(deps Deps) server.ToolHandlerFunc {
	return func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		if deps.CFClient == nil {
			return cfNotInstalledResult(deps), nil
		}
		domains, err := deps.CFClient.Domains(ctx)
		if err != nil {
			return handleCFError(err, deps), nil
		}
		limit := clampLimit(req.GetInt("limit", 20), 20, 100)
		total := len(domains)
		if limit < total {
			domains = domains[:limit]
		}
		return wrapResults(domains, total, len(domains), "domains", ""), nil
	}
}

func cfBuildpacksHandler(deps Deps) server.ToolHandlerFunc {
	return func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		if deps.CFClient == nil {
			return cfNotInstalledResult(deps), nil
		}
		bps, err := deps.CFClient.Buildpacks(ctx)
		if err != nil {
			return handleCFError(err, deps), nil
		}
		limit := clampLimit(req.GetInt("limit", 20), 20, 100)
		total := len(bps)
		if limit < total {
			bps = bps[:limit]
		}
		return wrapResults(bps, total, len(bps), "buildpacks", ""), nil
	}
}

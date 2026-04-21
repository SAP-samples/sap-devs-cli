package mcpserver

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
	"github.com/SAP-samples/sap-devs-cli/internal/btpcli"
)

func registerBTPTools(s *server.MCPServer, deps Deps) {
	s.AddTool(
		mcp.NewTool("btp_target",
			mcp.WithDescription("Get current BTP target (subaccount, region, global account, trial flag, login status). Use to check the user's BTP CLI state before running other btp_ tools."),
		),
		btpTargetHandler(deps),
	)

	s.AddTool(
		mcp.NewTool("btp_subaccounts",
			mcp.WithDescription("List BTP subaccounts with name, subdomain, region, state, and parent directory."),
			mcp.WithNumber("limit",
				mcp.Description("Maximum number of results to return (default 20, max 100)"),
			),
		),
		btpSubaccountsHandler(deps),
	)

	s.AddTool(
		mcp.NewTool("btp_service_instances",
			mcp.WithDescription("List BTP service instances with name, service, plan, status, and creation date."),
			mcp.WithNumber("limit",
				mcp.Description("Maximum number of results to return (default 20, max 100)"),
			),
		),
		btpServiceInstancesHandler(deps),
	)

	s.AddTool(
		mcp.NewTool("btp_role_collections",
			mcp.WithDescription("List BTP role collections with name, description, and role count."),
			mcp.WithNumber("limit",
				mcp.Description("Maximum number of results to return (default 20, max 100)"),
			),
		),
		btpRoleCollectionsHandler(deps),
	)
}

func handleBTPError(err error, deps Deps) *mcp.CallToolResult {
	switch e := err.(type) {
	case *btpcli.AuthError:
		return btpAuthErrorResult(e)
	case *btpcli.NotInstalledError:
		return btpNotInstalledResult(deps)
	default:
		return mcp.NewToolResultError(err.Error())
	}
}

func btpAuthErrorResult(err *btpcli.AuthError) *mcp.CallToolResult {
	result := map[string]string{
		"error":   "not_authenticated",
		"cli":     "btp",
		"message": err.Message,
		"fix":     "Run: btp login",
		"hint":    "The btp CLI requires an active login session. After logging in, retry the command.",
	}
	b, _ := json.Marshal(result)
	return mcp.NewToolResultText(string(b))
}

func btpNotInstalledResult(deps Deps) *mcp.CallToolResult {
	install := ""
	for _, p := range deps.Packs {
		for _, t := range p.Tools {
			if t.ID == "btp-cli" {
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
		"cli":     "btp",
		"message": "BTP CLI is not installed.",
		"fix":     fmt.Sprintf("Install: %s", install),
		"hint":    "The btp CLI is required for BTP operations. Install it and run 'btp login' to authenticate.",
	}
	b, _ := json.Marshal(result)
	return mcp.NewToolResultText(string(b))
}

func btpTargetHandler(deps Deps) server.ToolHandlerFunc {
	return func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		if deps.BTPClient == nil {
			return btpNotInstalledResult(deps), nil
		}
		info, err := deps.BTPClient.Target(ctx)
		if err != nil {
			return handleBTPError(err, deps), nil
		}
		b, _ := json.Marshal(info)
		return mcp.NewToolResultText(string(b)), nil
	}
}

func btpSubaccountsHandler(deps Deps) server.ToolHandlerFunc {
	return func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		if deps.BTPClient == nil {
			return btpNotInstalledResult(deps), nil
		}
		subs, err := deps.BTPClient.Subaccounts(ctx)
		if err != nil {
			return handleBTPError(err, deps), nil
		}
		limit := clampLimit(req.GetInt("limit", 20), 20, 100)
		total := len(subs)
		if limit < total {
			subs = subs[:limit]
		}
		return wrapResults(subs, total, len(subs), "subaccounts", ""), nil
	}
}

func btpServiceInstancesHandler(deps Deps) server.ToolHandlerFunc {
	return func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		if deps.BTPClient == nil {
			return btpNotInstalledResult(deps), nil
		}
		instances, err := deps.BTPClient.ServiceInstances(ctx)
		if err != nil {
			return handleBTPError(err, deps), nil
		}
		limit := clampLimit(req.GetInt("limit", 20), 20, 100)
		total := len(instances)
		if limit < total {
			instances = instances[:limit]
		}
		return wrapResults(instances, total, len(instances), "service instances", ""), nil
	}
}

func btpRoleCollectionsHandler(deps Deps) server.ToolHandlerFunc {
	return func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		if deps.BTPClient == nil {
			return btpNotInstalledResult(deps), nil
		}
		rcs, err := deps.BTPClient.RoleCollections(ctx)
		if err != nil {
			return handleBTPError(err, deps), nil
		}
		limit := clampLimit(req.GetInt("limit", 20), 20, 100)
		total := len(rcs)
		if limit < total {
			rcs = rcs[:limit]
		}
		return wrapResults(rcs, total, len(rcs), "role collections", ""), nil
	}
}

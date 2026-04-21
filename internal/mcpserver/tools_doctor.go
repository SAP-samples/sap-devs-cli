package mcpserver

import (
	"context"
	"encoding/json"
	"fmt"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/SAP-samples/sap-devs-cli/internal/content"
	"github.com/SAP-samples/sap-devs-cli/internal/project"
	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

func registerDoctorTools(s *server.MCPServer, deps Deps) {
	s.AddTool(
		mcp.NewTool("check_tools",
			mcp.WithDescription("Check which SAP developer tools are installed and their versions. Returns status (ok/fail/missing) with install commands for missing tools. Use when a user encounters 'command not found' errors or needs environment setup help."),
			mcp.WithNumber("limit",
				mcp.Description("Maximum number of results to return (default 20, max 100)"),
			),
		),
		checkToolsHandler(deps),
	)

	s.AddTool(
		mcp.NewTool("check_project",
			mcp.WithDescription("Run health checks on the current SAP project. Detects project type (CAP, MTA, UI5), checks dependencies, version staleness, and best-practice compliance. Returns findings with severity and fix suggestions. Use proactively when helping with SAP project issues."),
			mcp.WithString("path",
				mcp.Description("Absolute path to project root directory. If omitted, uses the working directory the MCP server was launched from."),
			),
		),
		checkProjectHandler(deps),
	)
}

type toolCheckResult struct {
	ID       string `json:"id"`
	Name     string `json:"name"`
	Status   string `json:"status"`
	Required string `json:"required"`
	Found    string `json:"found,omitempty"`
	Install  string `json:"install,omitempty"`
	Docs     string `json:"docs,omitempty"`
}

func execRunner(command string) (string, error) {
	parts := strings.Fields(command)
	if len(parts) == 0 {
		return "", fmt.Errorf("empty command")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, parts[0], parts[1:]...)
	out, err := cmd.CombinedOutput()
	return string(out), err
}

func installForCurrentOS(install map[string]string) string {
	goos := runtime.GOOS
	if cmd, ok := install[goos]; ok {
		return cmd
	}
	if goos == "darwin" {
		if cmd, ok := install["macos"]; ok {
			return cmd
		}
	}
	if cmd, ok := install["all"]; ok {
		return cmd
	}
	return ""
}

func checkToolsHandler(deps Deps) server.ToolHandlerFunc {
	return func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		limit := clampLimit(req.GetInt("limit", 20), 20, 100)

		var tools []content.ToolDef
		for _, p := range deps.Packs {
			tools = append(tools, p.Tools...)
		}
		results := content.CheckTools(tools, execRunner)
		total := len(results)
		if limit < total {
			results = results[:limit]
		}

		out := make([]toolCheckResult, 0, len(results))
		for _, r := range results {
			out = append(out, toolCheckResult{
				ID:       r.Tool.ID,
				Name:     r.Tool.Name,
				Status:   string(r.Status),
				Required: r.Tool.Required,
				Found:    r.Found,
				Install:  installForCurrentOS(r.Tool.Install),
				Docs:     r.Tool.Docs,
			})
		}
		return wrapResults(out, total, len(out), "tools", ""), nil
	}
}

type projectCheckResult struct {
	Detection projectDetection `json:"detection"`
	Findings  ResultEnvelope   `json:"findings"`
}

type projectDetection struct {
	Type          string `json:"type,omitempty"`
	CAPVersion    string `json:"cap_version,omitempty"`
	Database      string `json:"database,omitempty"`
	Deployment    string `json:"deployment,omitempty"`
	Auth          string `json:"auth,omitempty"`
	BTPSubaccount string `json:"btp_subaccount,omitempty"`
	BTPRegion     string `json:"btp_region,omitempty"`
	CFOrg         string `json:"cf_org,omitempty"`
	CFSpace       string `json:"cf_space,omitempty"`
}

type findingResult struct {
	Category string `json:"category"`
	Severity string `json:"severity"`
	Message  string `json:"message"`
	Fix      string `json:"fix,omitempty"`
}

func checkProjectHandler(deps Deps) server.ToolHandlerFunc {
	return func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		cwd := req.GetString("path", "")
		if cwd == "" {
			cwd = deps.Cwd
		}
		if cwd == "" {
			return mcp.NewToolResultError("no project path available"), nil
		}
		if !filepath.IsAbs(cwd) {
			return mcp.NewToolResultError("path must be an absolute path"), nil
		}

		pctx, err := project.Detect(cwd)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("project detection failed: %v", err)), nil
		}

		detection := projectDetection{
			Type:          pctx.Type,
			CAPVersion:    pctx.CAPVersion,
			Database:      pctx.Database,
			Deployment:    pctx.Deployment,
			Auth:          pctx.Auth,
			BTPSubaccount: pctx.BTPSubaccount,
			BTPRegion:     pctx.BTPRegion,
			CFOrg:         pctx.CFOrg,
			CFSpace:       pctx.CFSpace,
		}

		findings := project.Check(pctx, cwd, deps.Packs)
		out := make([]findingResult, 0, len(findings))
		for _, f := range findings {
			out = append(out, findingResult{
				Category: f.Category,
				Severity: f.Severity,
				Message:  f.Message,
				Fix:      f.Fix,
			})
		}

		result := projectCheckResult{
			Detection: detection,
			Findings: ResultEnvelope{
				Count:   len(out),
				Total:   len(out),
				Results: out,
			},
		}
		if len(out) == 0 {
			result.Findings.Hint = "No issues found — project looks healthy."
		} else {
			result.Findings.Hint = fmt.Sprintf("%d issue(s) found. Review the findings and apply suggested fixes.", len(out))
		}
		b, err := json.Marshal(result)
		if err != nil {
			return mcp.NewToolResultError("failed to serialize project check results"), nil
		}
		return mcp.NewToolResultText(string(b)), nil
	}
}

package mcpserver

import (
	"github.com/mark3labs/mcp-go/server"
	"github.com/SAP-samples/sap-devs-cli/internal/content"
	"github.com/SAP-samples/sap-devs-cli/internal/learning"
	"github.com/SAP-samples/sap-devs-cli/internal/tutorials"
)

// Deps holds all dependencies required to construct and operate the MCP server.
type Deps struct {
	Packs         []*content.Pack
	Profile       *content.Profile
	TutorialIndex []tutorials.TutorialMeta
	LearningIndex []learning.LearningJourney
	CacheDir      string
	ConfigDir     string
	Version       string
}

// NewServer creates a new MCP server with all SAP developer tools registered.
func NewServer(deps Deps) *server.MCPServer {
	s := server.NewMCPServer(
		"sap-devs",
		deps.Version,
		server.WithToolCapabilities(false),
		server.WithInstructions("SAP developer knowledge server. Use these tools to get SAP-specific context, tips, resources, error patterns, news, tutorials, and learning journeys on demand."),
	)

	registerContentTools(s, deps)
	registerResourceTools(s, deps)
	registerErrorTools(s, deps)
	registerNewsTools(s, deps)
	registerLearnTools(s, deps)
	registerSampleTools(s, deps)

	return s
}

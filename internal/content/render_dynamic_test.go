package content_test

import (
	"strings"
	"testing"

	"github.tools.sap/developer-relations/sap-devs-cli/internal/content"
)

func TestRenderDynamic_ProjectContext(t *testing.T) {
	pc := &content.ProjectInfo{
		Type:       "CAP (Node.js)",
		CAPVersion: "9.6.2",
		Facts: []content.ProjectFact{
			{Key: "Project type", Value: "CAP (Node.js)"},
			{Key: "CAP version", Value: "@sap/cds 9.6.2 (latest: 9.8.0)", Warn: "update available: 9.8.0"},
			{Key: "Database", Value: "SAP HANA Cloud"},
			{Key: "Deployment", Value: "MTA to Cloud Foundry"},
			{Key: "Auth", Value: "XSUAA (xs-security.json detected)"},
		},
	}
	d := &content.DynamicContext{
		CLIVersion: "1.5.0",
		Project:    pc,
	}

	out := content.RenderDynamic(d)

	if !strings.Contains(out, "**Project Context (detected):**") {
		t.Error("missing Project Context header")
	}
	if !strings.Contains(out, "CAP version") {
		t.Error("missing CAP version fact")
	}
	if !strings.Contains(out, "SAP HANA Cloud") {
		t.Error("missing database fact")
	}
}

func TestRenderDynamic_NoProject(t *testing.T) {
	d := &content.DynamicContext{CLIVersion: "1.5.0"}

	out := content.RenderDynamic(d)

	if strings.Contains(out, "Project Context") {
		t.Error("should not render project section when no project detected")
	}
}

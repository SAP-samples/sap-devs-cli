package content_test

import (
	"strings"
	"testing"

	"github.com/SAP-samples/sap-devs-cli/internal/content"
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

func TestRenderDynamic_BTPEnvironment(t *testing.T) {
	d := &content.DynamicContext{
		CLIVersion: "1.5.0",
		Project: &content.ProjectInfo{
			Type: "CAP (Node.js)",
			Facts: []content.ProjectFact{
				{Key: "Project type", Value: "CAP (Node.js)"},
			},
			BTPFacts: []content.ProjectFact{
				{Key: "BTP subaccount", Value: "trial-abc (eu10, trial)"},
				{Key: "Cloud Foundry", Value: "MyOrg/dev (us10)"},
			},
		},
	}

	out := content.RenderDynamic(d)

	if !strings.Contains(out, "**BTP Environment (detected):**") {
		t.Error("missing BTP Environment header")
	}
	if !strings.Contains(out, "BTP subaccount: trial-abc (eu10, trial)") {
		t.Error("missing BTP subaccount fact")
	}
	if !strings.Contains(out, "Cloud Foundry: MyOrg/dev (us10)") {
		t.Error("missing Cloud Foundry fact")
	}
	if !strings.Contains(out, "**Project Context (detected):**") {
		t.Error("project facts should still render")
	}
}

func TestRenderDynamic_BTPOnly_NoProjectFacts(t *testing.T) {
	d := &content.DynamicContext{
		CLIVersion: "1.5.0",
		Project: &content.ProjectInfo{
			BTPFacts: []content.ProjectFact{
				{Key: "Cloud Foundry", Value: "MyOrg/dev (us10)"},
			},
		},
	}

	out := content.RenderDynamic(d)

	if !strings.Contains(out, "**BTP Environment (detected):**") {
		t.Error("missing BTP Environment header")
	}
	if strings.Contains(out, "**Project Context (detected):**") {
		t.Error("should not render project context header when no project facts")
	}
}

func TestRenderDynamic_NoBTP(t *testing.T) {
	d := &content.DynamicContext{
		CLIVersion: "1.5.0",
		Project: &content.ProjectInfo{
			Type: "CAP (Node.js)",
			Facts: []content.ProjectFact{
				{Key: "Project type", Value: "CAP (Node.js)"},
			},
		},
	}

	out := content.RenderDynamic(d)

	if strings.Contains(out, "BTP Environment") {
		t.Error("should not render BTP section when no BTP facts")
	}
}

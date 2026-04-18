package tutorials_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.tools.sap/developer-relations/sap-devs-cli/internal/tutorials"
)

const sampleV2Markdown = `---
parser: v2
time: 20
tags: [ tutorial>beginner, software-product>sap-cloud-application-programming-model ]
primary_tag: software-product>sap-cloud-application-programming-model
author_name: Test Author
---

# Getting Started with CAP
<!-- description --> Learn how to create your first CAP project

## Prerequisites
- Node.js 18+
- @sap/cds-dk installed

## You will learn
- How to init a CAP project
- How to define a data model

### Create a new CAP project
Open a terminal and run:
` + "```" + `bash
cds init my-bookshop
` + "```" + `

### Define a data model
Create ` + "`" + `db/schema.cds` + "`" + `:
` + "```" + `cds
entity Books { key ID : Integer; title : String; }
` + "```" + `
`

func TestParseV2_BasicStructure(t *testing.T) {
	tut, err := tutorials.Parse(sampleV2Markdown, "cap-getting-started", "Tutorials", "main")
	require.NoError(t, err)
	assert.Equal(t, "Getting Started with CAP", tut.Title)
	assert.Equal(t, "Learn how to create your first CAP project", tut.Description)
	assert.Equal(t, 20, tut.Time)
	assert.Equal(t, "beginner", tut.Level)
	assert.Equal(t, "Test Author", tut.Author)
	assert.Contains(t, tut.Prerequisites, "Node.js 18+")
	require.Len(t, tut.YouWillLearn, 2)
	assert.Equal(t, "How to init a CAP project", tut.YouWillLearn[0])
	require.Len(t, tut.Steps, 2)
	assert.Equal(t, 1, tut.Steps[0].Number)
	assert.Equal(t, "Create a new CAP project", tut.Steps[0].Title)
	assert.Contains(t, tut.Steps[0].Content, "cds init my-bookshop")
	assert.Equal(t, 2, tut.Steps[1].Number)
	assert.Equal(t, "Define a data model", tut.Steps[1].Title)
}

func TestParseV2_TitleFromFrontmatter(t *testing.T) {
	md := "---\nparser: v2\ntitle: Explicit Title\ntime: 10\ntags: [tutorial>advanced]\nprimary_tag: topic>cloud\n---\n\n### Step One\nContent\n"
	tut, err := tutorials.Parse(md, "test-slug", "Tutorials", "main")
	require.NoError(t, err)
	assert.Equal(t, "Explicit Title", tut.Title)
}

func TestParseV2_URLGeneration(t *testing.T) {
	md := "---\nparser: v2\ntime: 10\ntags: [tutorial>beginner]\nprimary_tag: topic>cloud\n---\n\n# Title\n\n### Step\nContent\n"
	tut, err := tutorials.Parse(md, "my-tutorial", "Tutorials", "main")
	require.NoError(t, err)
	assert.Equal(t, "https://developers.sap.com/tutorials/my-tutorial.html", tut.URL)
}

const sampleV1Markdown = `---
time: 15
tags: [ tutorial>intermediate ]
primary_tag: topic>abap
---

# Legacy Tutorial

[ACCORDION-BEGIN [Step 1: ](Create Something)]
Do this first.
[ACCORDION-END]

[ACCORDION-BEGIN [Step 2: ](Configure Something)]
Then do this.
[ACCORDION-END]
`

func TestParseV1_AccordionSteps(t *testing.T) {
	tut, err := tutorials.Parse(sampleV1Markdown, "legacy-tutorial", "Tutorials", "main")
	require.NoError(t, err)
	assert.Equal(t, "Legacy Tutorial", tut.Title)
	assert.Equal(t, "intermediate", tut.Level)
	require.Len(t, tut.Steps, 2)
	assert.Equal(t, "Create Something", tut.Steps[0].Title)
	assert.Contains(t, tut.Steps[0].Content, "Do this first.")
	assert.Equal(t, "Configure Something", tut.Steps[1].Title)
}

func TestParseFrontmatterOnly(t *testing.T) {
	md := "---\nparser: v2\ntime: 30\ntags: [tutorial>beginner, topic>cloud]\nprimary_tag: topic>cloud\nauthor_name: Jane Doe\n---\n\n# My Tutorial\n<!-- description --> A description\n\n### Step 1\nContent\n"
	meta, err := tutorials.ParseFrontmatterOnly(md, "my-slug", "TestRepo", "main")
	require.NoError(t, err)
	assert.Equal(t, "my-slug", meta.Slug)
	assert.Equal(t, "My Tutorial", meta.Title)
	assert.Equal(t, "A description", meta.Description)
	assert.Equal(t, 30, meta.Time)
	assert.Equal(t, "beginner", meta.Level)
	assert.Equal(t, "Jane Doe", meta.Author)
	assert.Equal(t, "TestRepo", meta.Repo)
	assert.Equal(t, "v2", meta.Parser)
}

func TestParseFrontmatterOnly_SlugFallbackTitle(t *testing.T) {
	md := "---\ntime: 10\ntags: [tutorial>beginner]\nprimary_tag: x\n---\nNo headings here.\n"
	meta, err := tutorials.ParseFrontmatterOnly(md, "my-slug", "Repo", "main")
	require.NoError(t, err)
	assert.Equal(t, "my-slug", meta.Title)
}

func TestParseV2_OptionBlocks(t *testing.T) {
	md := "---\nparser: v2\ntime: 10\ntags: [tutorial>beginner]\nprimary_tag: x\n---\n\n# Test\n\n### Install dependencies\n\n[OPTION BEGIN [Node.js]]\nRun `npm install`.\n[OPTION END]\n\n[OPTION BEGIN [Java]]\nRun `mvn install`.\n[OPTION END]\n"
	tut, err := tutorials.Parse(md, "test-options", "Tutorials", "main")
	require.NoError(t, err)
	require.Len(t, tut.Steps, 1)
	assert.Contains(t, tut.Steps[0].Content, "#### Option: Node.js")
	assert.Contains(t, tut.Steps[0].Content, "#### Option: Java")
	assert.NotContains(t, tut.Steps[0].Content, "[OPTION BEGIN")
	assert.NotContains(t, tut.Steps[0].Content, "[OPTION END]")
}

func TestExtractLevel(t *testing.T) {
	tests := []struct {
		tags  []string
		level string
	}{
		{[]string{"tutorial>beginner", "topic>cloud"}, "beginner"},
		{[]string{"tutorial>intermediate"}, "intermediate"},
		{[]string{"tutorial>advanced"}, "advanced"},
		{[]string{"topic>cloud"}, ""},
	}
	for _, tt := range tests {
		assert.Equal(t, tt.level, tutorials.ExtractLevel(tt.tags))
	}
}

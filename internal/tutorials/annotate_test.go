package tutorials_test

import (
	"testing"

	"github.com/SAP-samples/sap-devs-cli/internal/tutorials"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAnnotateStep_SingleCommand(t *testing.T) {
	md := "Run the following command:\n\n```bash\ncds init bookshop\n```\n"
	ann := tutorials.AnnotateStep(md)
	assert.Len(t, ann.Commands, 1)
	assert.Equal(t, "cds init bookshop", ann.Commands[0].Command)
	assert.Contains(t, ann.Commands[0].Description, "Run the following command")
}

func TestAnnotateStep_MultiLineCommands(t *testing.T) {
	md := "Install dependencies:\n\n```bash\nnpm install\nnpm start\n```\n"
	ann := tutorials.AnnotateStep(md)
	assert.Len(t, ann.Commands, 2)
	assert.Equal(t, "npm install", ann.Commands[0].Command)
	assert.Equal(t, "npm start", ann.Commands[1].Command)
}

func TestAnnotateStep_SkipOutputBlock(t *testing.T) {
	md := "You should see the following output:\n\n```bash\nServer running at http://localhost:4004\n```\n"
	ann := tutorials.AnnotateStep(md)
	assert.Empty(t, ann.Commands)
}

func TestAnnotateStep_SkipCommentOnlyBlock(t *testing.T) {
	md := "Example:\n\n```bash\n# This is just a comment\n```\n"
	ann := tutorials.AnnotateStep(md)
	assert.Empty(t, ann.Commands)
}

func TestAnnotateStep_NoLanguageTag(t *testing.T) {
	md := "Run this:\n\n```\nnpm install\n```\n"
	ann := tutorials.AnnotateStep(md)
	assert.Len(t, ann.Commands, 1)
	assert.Equal(t, "npm install", ann.Commands[0].Command)
}

func TestAnnotateStep_ShTag(t *testing.T) {
	md := "Execute:\n\n```sh\ncds watch\n```\n"
	ann := tutorials.AnnotateStep(md)
	assert.Len(t, ann.Commands, 1)
	assert.Equal(t, "cds watch", ann.Commands[0].Command)
}

func TestAnnotateStep_EmptyStep(t *testing.T) {
	ann := tutorials.AnnotateStep("")
	assert.Empty(t, ann.Commands)
	assert.Empty(t, ann.FileCreates)
	assert.Empty(t, ann.Verifications)
}

func TestAnnotateStep_TextOnly(t *testing.T) {
	md := "This step has no code blocks, just explanatory text.\n\nRead the documentation carefully.\n"
	ann := tutorials.AnnotateStep(md)
	assert.Empty(t, ann.Commands)
	assert.Empty(t, ann.FileCreates)
	assert.Empty(t, ann.Verifications)
}

func TestAnnotateStep_FileCreate(t *testing.T) {
	md := "Create the file `db/schema.cds`:\n\n```cds\nentity Books { key ID : Integer; title : String; }\n```\n"
	ann := tutorials.AnnotateStep(md)
	assert.Empty(t, ann.Commands)
	assert.Len(t, ann.FileCreates, 1)
	assert.Equal(t, "db/schema.cds", ann.FileCreates[0].Filename)
	assert.Equal(t, "cds", ann.FileCreates[0].Language)
	assert.Contains(t, ann.FileCreates[0].Content, "entity Books")
}

func TestAnnotateStep_FileCreate_NoFilename(t *testing.T) {
	md := "Here is an example CDS model:\n\n```cds\nentity Foo { key ID : Integer; }\n```\n"
	ann := tutorials.AnnotateStep(md)
	assert.Empty(t, ann.FileCreates)
}

func TestAnnotateStep_Verification(t *testing.T) {
	md := "You should see the following output:\n\n```\nServer running at http://localhost:4004\n```\n"
	ann := tutorials.AnnotateStep(md)
	assert.Empty(t, ann.Commands)
	assert.Len(t, ann.Verifications, 1)
	assert.Contains(t, ann.Verifications[0].ExpectOutput, "localhost:4004")
}

func TestAnnotateStep_MixedContent(t *testing.T) {
	md := "Install the dependency:\n\n```bash\nnpm install @sap/cds\n```\n\n" +
		"Create the file `srv/service.cds`:\n\n```cds\nservice CatalogService { entity Books as projection on my.Books; }\n```\n\n" +
		"Verify the installation:\n\n```bash\ncds version\n```\n"
	ann := tutorials.AnnotateStep(md)
	assert.Len(t, ann.Commands, 1)
	assert.Equal(t, "npm install @sap/cds", ann.Commands[0].Command)
	assert.Len(t, ann.FileCreates, 1)
	assert.Equal(t, "srv/service.cds", ann.FileCreates[0].Filename)
	assert.Len(t, ann.Verifications, 1)
	assert.Contains(t, ann.Verifications[0].ExpectOutput, "cds version")
}

func TestAnnotateStep_FileCreate_EditAction(t *testing.T) {
	md := "Edit `package.json` and add:\n\n```json\n{\"dependencies\": {}}\n```\n"
	ann := tutorials.AnnotateStep(md)
	assert.Len(t, ann.FileCreates, 1)
	assert.Equal(t, "package.json", ann.FileCreates[0].Filename)
}

func TestAnnotateStep_WorkingDir(t *testing.T) {
	md := "Change to the project directory:\n\n```bash\ncd bookshop\nnpm install\n```\n"
	ann := tutorials.AnnotateStep(md)
	assert.Len(t, ann.Commands, 2)
	assert.Equal(t, "cd bookshop", ann.Commands[0].Command)
	assert.Equal(t, "npm install", ann.Commands[1].Command)
}

func TestAnnotateStep_VerificationConfidence(t *testing.T) {
	md := "You should see the following output:\n\n```\nserver running on port 4004\n```"
	ann := tutorials.AnnotateStep(md)
	require.Len(t, ann.Verifications, 1)
	assert.Equal(t, "high", ann.Verifications[0].Confidence)
}

func TestAnnotateStep_PrerequisiteTools(t *testing.T) {
	md := "Make sure you have **Node.js** installed.\n\nAlso ensure you have the `cf` CLI installed.\n\n```bash\nnpm i -g @sap/cds-dk\n```"
	ann := tutorials.AnnotateStep(md)
	require.Len(t, ann.PrerequisiteTools, 2)
	assert.Contains(t, ann.PrerequisiteTools, "Node.js")
	assert.Contains(t, ann.PrerequisiteTools, "cf")
}

func TestAnnotateStep_NoPrerequisites(t *testing.T) {
	md := "Run the following command:\n\n```bash\ncds init bookshop\n```"
	ann := tutorials.AnnotateStep(md)
	assert.Empty(t, ann.PrerequisiteTools)
}

package tutorials_test

import (
	"testing"

	"github.com/SAP-samples/sap-devs-cli/internal/tutorials"
	"github.com/stretchr/testify/assert"
)

func TestExtractImageRefs_RelativePaths(t *testing.T) {
	content := "Some text\n![cds commands](cds_commands.png)\nMore text\n![folder](folder_structure.png)\n"
	refs := tutorials.ExtractImageRefs(content, "btp-adai", "main", "cp-apm-nodejs-create-service")
	assert.Len(t, refs, 2)
	assert.Equal(t, "cds commands", refs[0].Alt)
	assert.Equal(t, "cds_commands.png", refs[0].OriginalPath)
	assert.Equal(t, "https://raw.githubusercontent.com/sap-tutorials/btp-adai/main/tutorials/cp-apm-nodejs-create-service/cds_commands.png", refs[0].URL)
}

func TestExtractImageRefs_AbsoluteURLs(t *testing.T) {
	content := "![logo](https://example.com/logo.png)\n"
	refs := tutorials.ExtractImageRefs(content, "repo", "main", "slug")
	assert.Len(t, refs, 1)
	assert.Equal(t, "https://example.com/logo.png", refs[0].URL)
	assert.Equal(t, "https://example.com/logo.png", refs[0].OriginalPath)
}

func TestExtractImageRefs_SkipsTraversals(t *testing.T) {
	content := "![bad](../../secret.png)\n"
	refs := tutorials.ExtractImageRefs(content, "repo", "main", "slug")
	assert.Len(t, refs, 1)
	assert.Equal(t, "../../secret.png", refs[0].URL) // not resolved
}

func TestExtractImageRefs_NoImages(t *testing.T) {
	content := "Just text, no images here.\n"
	refs := tutorials.ExtractImageRefs(content, "repo", "main", "slug")
	assert.Empty(t, refs)
}

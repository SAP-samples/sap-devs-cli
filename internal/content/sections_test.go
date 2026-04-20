package content_test

import (
	"bytes"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/SAP-samples/sap-devs-cli/internal/content"
)

func captureStderr(fn func()) string {
	old := os.Stderr
	r, w, _ := os.Pipe()
	os.Stderr = w
	fn()
	w.Close()
	var buf bytes.Buffer
	buf.ReadFrom(r)
	os.Stderr = old
	return buf.String()
}

func TestValidateContextSections_AllRecognized(t *testing.T) {
	md := "## SAP CAP\n\n### Overview\nIntro.\n\n### Key Concepts\nStuff.\n\n### Best Practices\nDo this.\n"
	out := captureStderr(func() { content.ValidateContextSections("cap", md) })
	assert.Empty(t, out)
}

func TestValidateContextSections_UnrecognizedSection(t *testing.T) {
	md := "## SAP CAP\n\n### Overview\nIntro.\n\n### Foo Bar\nCustom.\n"
	out := captureStderr(func() { content.ValidateContextSections("cap", md) })
	assert.Contains(t, out, `unrecognized section "Foo Bar"`)
	assert.Contains(t, out, `"cap"`)
}

func TestValidateContextSections_OutOfOrder(t *testing.T) {
	md := "## SAP CAP\n\n### Best Practices\nDo this.\n\n### Overview\nIntro.\n"
	out := captureStderr(func() { content.ValidateContextSections("cap", md) })
	assert.Contains(t, out, `out of order`)
	assert.Contains(t, out, `"Overview"`)
}

func TestValidateContextSections_NoH3Sections(t *testing.T) {
	md := "## SAP CAP\n\nJust a paragraph with no subsections.\n"
	out := captureStderr(func() { content.ValidateContextSections("cap", md) })
	assert.Empty(t, out)
}

func TestValidateContextSections_EmptyInput(t *testing.T) {
	out := captureStderr(func() { content.ValidateContextSections("test", "") })
	assert.Empty(t, out)
}

func TestValidateContextSections_MixedRecognizedAndCustom(t *testing.T) {
	md := "## ABAP\n\n### Overview\nIntro.\n\n### RAP Quick Reference\nCustom.\n\n### Best Practices\nDo this.\n"
	out := captureStderr(func() { content.ValidateContextSections("abap", md) })
	assert.Contains(t, out, `unrecognized section "RAP Quick Reference"`)
	assert.NotContains(t, out, `out of order`)
}

func TestValidateContextSections_VerbosityMarkersIgnored(t *testing.T) {
	md := "## CAP\n\n### Overview\nIntro.\n<!-- verbosity:detail -->\n### Anti-patterns\nDon't.\n"
	out := captureStderr(func() { content.ValidateContextSections("cap", md) })
	assert.Empty(t, out)
}

func TestValidateContextSections_AllFiveSectionsInOrder(t *testing.T) {
	md := "## Pack\n\n### Overview\nA.\n\n### Key Concepts\nB.\n\n### Best Practices\nC.\n\n### Anti-patterns\nD.\n\n### Code Examples\nE.\n"
	out := captureStderr(func() { content.ValidateContextSections("full", md) })
	assert.Empty(t, out)
}

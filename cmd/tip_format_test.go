package cmd_test

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.tools.sap/developer-relations/sap-devs-cli/cmd"
	"github.tools.sap/developer-relations/sap-devs-cli/internal/content"
)

func TestFormatTip_Markdown(t *testing.T) {
	tip := content.Tip{Title: "Use cds watch", Content: "Run cds watch for live reload."}
	out := cmd.FormatTip(tip, true, false)
	assert.True(t, strings.HasPrefix(out, "## 💡 Use cds watch"), "must start with ## 💡")
	assert.NotContains(t, out, "\x1b[", "must have no ANSI escape sequences")
}

func TestFormatTip_Plain(t *testing.T) {
	tip := content.Tip{Title: "Use cds watch", Content: "Run cds watch for live reload."}
	out := cmd.FormatTip(tip, false, true)
	assert.False(t, strings.HasPrefix(out, "#"), "must not start with a heading")
	assert.NotContains(t, out, "\x1b[", "must have no ANSI escape sequences")
	assert.Contains(t, out, "Use cds watch")
}

func TestFormatTip_DefaultReturnsEmpty(t *testing.T) {
	tip := content.Tip{Title: "Use cds watch", Content: "Run cds watch for live reload."}
	out := cmd.FormatTip(tip, false, false)
	assert.Empty(t, out, "default format returns empty string (caller uses glamour)")
}

package content_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.tools.sap/developer-relations/sap-devs-cli/internal/content"
)

func TestParseVerbositySections_NoMarkers(t *testing.T) {
	md := "## Title\n\nAll content here.\n"
	v := content.ParseVerbositySections(md)
	assert.Equal(t, md, v.Core)
	assert.Empty(t, v.Detail)
	assert.Empty(t, v.Extended)
}

func TestParseVerbositySections_AllThreeTiers(t *testing.T) {
	md := "Core content.\n<!-- verbosity:detail -->\nDetail content.\n<!-- verbosity:extended -->\nExtended content.\n"
	v := content.ParseVerbositySections(md)
	assert.Equal(t, "Core content.\n", v.Core)
	assert.Equal(t, "Detail content.\n", v.Detail)
	assert.Equal(t, "Extended content.\n", v.Extended)
}

func TestParseVerbositySections_CoreReset(t *testing.T) {
	md := "A.\n<!-- verbosity:detail -->\nB.\n<!-- verbosity:core -->\nC.\n"
	v := content.ParseVerbositySections(md)
	assert.Equal(t, "A.\nC.\n", v.Core)
	assert.Equal(t, "B.\n", v.Detail)
	assert.Empty(t, v.Extended)
}

func TestParseVerbositySections_AdjacentMarkers(t *testing.T) {
	md := "Core.\n<!-- verbosity:detail -->\n<!-- verbosity:extended -->\nExtended.\n"
	v := content.ParseVerbositySections(md)
	assert.Equal(t, "Core.\n", v.Core)
	assert.Empty(t, v.Detail)
	assert.Equal(t, "Extended.\n", v.Extended)
}

func TestParseVerbositySections_EmptyInput(t *testing.T) {
	v := content.ParseVerbositySections("")
	assert.Empty(t, v.Core)
	assert.Empty(t, v.Detail)
	assert.Empty(t, v.Extended)
}

func TestParseVerbositySections_MarkersStrippedFromOutput(t *testing.T) {
	md := "Core.\n<!-- verbosity:detail -->\nDetail.\n"
	v := content.ParseVerbositySections(md)
	assert.NotContains(t, v.Core, "<!-- verbosity")
	assert.NotContains(t, v.Detail, "<!-- verbosity")
}

func TestVerbositySections_AtLevel_Minimal(t *testing.T) {
	v := content.VerbositySections{Core: "C.", Detail: "D.", Extended: "E."}
	assert.Equal(t, "C.", v.AtLevel("minimal"))
}

func TestVerbositySections_AtLevel_Standard(t *testing.T) {
	v := content.VerbositySections{Core: "C.", Detail: "D.", Extended: "E."}
	assert.Equal(t, "C.D.", v.AtLevel("standard"))
}

func TestVerbositySections_AtLevel_Full(t *testing.T) {
	v := content.VerbositySections{Core: "C.", Detail: "D.", Extended: "E."}
	assert.Equal(t, "C.D.E.", v.AtLevel("full"))
}

func TestVerbositySections_AtLevel_EmptyDefault(t *testing.T) {
	v := content.VerbositySections{Core: "C.", Detail: "D.", Extended: "E."}
	assert.Equal(t, "C.D.E.", v.AtLevel(""), "empty string defaults to full")
}

func TestVerbositySections_AtLevel_EmptyTiers(t *testing.T) {
	v := content.VerbositySections{Core: "C.", Detail: "", Extended: "E."}
	assert.Equal(t, "C.E.", v.AtLevel("full"))
	assert.Equal(t, "C.", v.AtLevel("standard"))
}

func TestParseVerbositySections_UnknownMarkerTreatedAsCore(t *testing.T) {
	md := "A.\n<!-- verbosity:bogus -->\nB.\n"
	v := content.ParseVerbositySections(md)
	assert.Equal(t, "A.\nB.\n", v.Core, "unknown marker content falls into core")
	assert.Empty(t, v.Detail)
	assert.Empty(t, v.Extended)
}

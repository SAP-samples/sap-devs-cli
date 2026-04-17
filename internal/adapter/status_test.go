package adapter_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.tools.sap/developer-relations/sap-devs-cli/internal/adapter"
)

func TestEstimateTokens_Empty(t *testing.T) {
	assert.Equal(t, 0, adapter.EstimateTokens(""))
}

func TestEstimateTokens_KnownString(t *testing.T) {
	// "hello world foo bar" = 4 words → 4 * 13 / 10 = 5
	assert.Equal(t, 5, adapter.EstimateTokens("hello world foo bar"))
}

func TestScanOtherSections_Empty(t *testing.T) {
	result := adapter.ScanOtherSections("")
	assert.NotNil(t, result)
	assert.Empty(t, result)
}

func TestScanOtherSections_IgnoresSapDevs(t *testing.T) {
	content := "<!-- sap-devs:start:SAP Dev -->\nhello\n<!-- sap-devs:end:SAP Dev -->\n"
	result := adapter.ScanOtherSections(content)
	assert.Empty(t, result)
}

func TestScanOtherSections_OneMatch(t *testing.T) {
	content := "<!-- cursor:start:Rules -->\nsome cursor rules here\n<!-- cursor:end:Rules -->\n"
	result := adapter.ScanOtherSections(content)
	assert.Len(t, result, 1)
	assert.Equal(t, "cursor", result[0].Name)
	assert.Greater(t, result[0].Tokens, 0)
}

func TestScanOtherSections_MultipleTools(t *testing.T) {
	content := "<!-- cursor:start:Rules -->\ncursor rules\n<!-- cursor:end:Rules -->\n<!-- copilot:start:Instructions -->\ncopilot stuff\n<!-- copilot:end:Instructions -->\n"
	result := adapter.ScanOtherSections(content)
	assert.Len(t, result, 2)
	names := []string{result[0].Name, result[1].Name}
	assert.Contains(t, names, "cursor")
	assert.Contains(t, names, "copilot")
}

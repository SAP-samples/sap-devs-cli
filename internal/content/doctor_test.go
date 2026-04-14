package content_test

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.tools.sap/developer-relations/sap-devs-cli/internal/content"
)

func TestParseConstraint_GTE_Satisfied(t *testing.T) {
	assert.True(t, content.ParseConstraintForTest(">=18.0.0", "18.0.0"))
	assert.True(t, content.ParseConstraintForTest(">=18.0.0", "v20.11.0"))
	assert.True(t, content.ParseConstraintForTest(">=18.0.0", "18.0.1"))
}

func TestParseConstraint_GTE_NotSatisfied(t *testing.T) {
	assert.False(t, content.ParseConstraintForTest(">=18.0.0", "17.9.9"))
	assert.False(t, content.ParseConstraintForTest(">=18.0.0", "v17.0.0"))
}

func TestParseConstraint_GT(t *testing.T) {
	assert.True(t, content.ParseConstraintForTest(">18.0.0", "18.0.1"))
	assert.False(t, content.ParseConstraintForTest(">18.0.0", "18.0.0"))
}

func TestParseConstraint_LTE(t *testing.T) {
	assert.True(t, content.ParseConstraintForTest("<=18.0.0", "18.0.0"))
	assert.True(t, content.ParseConstraintForTest("<=18.0.0", "17.9.9"))
	assert.False(t, content.ParseConstraintForTest("<=18.0.0", "18.0.1"))
}

func TestParseConstraint_LT(t *testing.T) {
	assert.True(t, content.ParseConstraintForTest("<18.0.0", "17.9.9"))
	assert.False(t, content.ParseConstraintForTest("<18.0.0", "18.0.0"))
}

func TestParseConstraint_EQ(t *testing.T) {
	assert.True(t, content.ParseConstraintForTest("=18.0.0", "18.0.0"))
	assert.False(t, content.ParseConstraintForTest("=18.0.0", "18.0.1"))
}

func TestParseConstraint_PartialVersion(t *testing.T) {
	assert.True(t, content.ParseConstraintForTest(">=8", "8.0.0"))
	assert.True(t, content.ParseConstraintForTest(">=8", "8.1.0"))
	assert.False(t, content.ParseConstraintForTest(">=8", "7.9.9"))
}

func TestParseConstraint_VersionWithSuffix(t *testing.T) {
	assert.True(t, content.ParseConstraintForTest(">=7.0.0", "7.9.3 (release)"))
	assert.True(t, content.ParseConstraintForTest(">=18.0.0", "v20.11.0-alpine3.19"))
}

func TestParseConstraint_UnrecognisedOperator(t *testing.T) {
	assert.False(t, content.ParseConstraintForTest("18.0.0", "18.0.0"))
}

func TestParseConstraint_UnparsableFound(t *testing.T) {
	assert.False(t, content.ParseConstraintForTest(">=18.0.0", "not-a-version"))
}

func TestParseConstraint_UnparsableRequired(t *testing.T) {
	assert.False(t, content.ParseConstraintForTest(">=not-a-version", "1.0.0"))
}

func fakeRunner(output string, err error) content.Runner {
	return func(command string) (string, error) {
		return output, err
	}
}

func toolDef(id, required, command, pattern string) content.ToolDef {
	return content.ToolDef{
		ID:       id,
		Name:     id,
		Required: required,
		Detect: content.ToolDetect{
			Command: command,
			Pattern: pattern,
		},
	}
}

func TestCheckTool_OK(t *testing.T) {
	tool := toolDef("node", ">=18.0.0", "node --version", `v(\d+\.\d+\.\d+)`)
	result := content.CheckTool(tool, fakeRunner("v20.11.0", nil))
	assert.Equal(t, content.StatusOK, result.Status)
	assert.Equal(t, "20.11.0", result.Found)
}

func TestCheckTool_Fail(t *testing.T) {
	tool := toolDef("cds", ">=7.0.0", "cds --version", `@sap/cds: (\d+\.\d+\.\d+)`)
	result := content.CheckTool(tool, fakeRunner("@sap/cds: 6.8.2\n", nil))
	assert.Equal(t, content.StatusFail, result.Status)
	assert.Equal(t, "6.8.2", result.Found)
}

func TestCheckTool_Missing_RunnerError(t *testing.T) {
	tool := toolDef("cf", ">=8.0.0", "cf --version", `cf version (\d+\.\d+\.\d+)`)
	result := content.CheckTool(tool, fakeRunner("", fmt.Errorf("not found")))
	assert.Equal(t, content.StatusMissing, result.Status)
	assert.Empty(t, result.Found)
}

func TestCheckTool_PatternNoMatch(t *testing.T) {
	tool := toolDef("cf", ">=8.0.0", "cf --version", `cf version (\d+\.\d+\.\d+)`)
	result := content.CheckTool(tool, fakeRunner("some unrelated output", nil))
	assert.Equal(t, content.StatusMissing, result.Status)
	assert.Empty(t, result.Found)
}

func TestCheckTool_Latest_Present(t *testing.T) {
	tool := toolDef("btp", "latest", "btp --version", `SAP BTP command line interface \(client v(\S+)\)`)
	result := content.CheckTool(tool, fakeRunner("SAP BTP command line interface (client v3.65.0)\n", nil))
	assert.Equal(t, content.StatusUnknown, result.Status)
	assert.Equal(t, "3.65.0", result.Found)
}

func TestCheckTool_Latest_Missing(t *testing.T) {
	tool := toolDef("btp", "latest", "btp --version", `SAP BTP command line interface \(client v(\S+)\)`)
	result := content.CheckTool(tool, fakeRunner("", fmt.Errorf("not found")))
	assert.Equal(t, content.StatusMissing, result.Status)
}

func TestCheckTools_Dedup(t *testing.T) {
	callCount := 0
	countingRunner := func(command string) (string, error) {
		callCount++
		return "v20.11.0", nil
	}
	tools := []content.ToolDef{
		toolDef("node", ">=18.0.0", "node --version", `v(\d+\.\d+\.\d+)`),
		toolDef("node", ">=18.0.0", "node --version", `v(\d+\.\d+\.\d+)`),
	}
	results := content.CheckTools(tools, countingRunner)
	assert.Len(t, results, 1)
	assert.Equal(t, 1, callCount)
	assert.Equal(t, content.StatusOK, results[0].Status)
}

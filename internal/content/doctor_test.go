package content_test

import (
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

package adapter_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.tools.sap/developer-relations/sap-devs-cli/internal/adapter"
)

func TestDetect_Empty(t *testing.T) {
	a := adapter.Adapter{ID: "test", Detect: nil}
	assert.False(t, adapter.Detect(a))
}

func TestDetect_PathRule_Exists(t *testing.T) {
	dir := t.TempDir()
	file := filepath.Join(dir, "tool")
	require.NoError(t, os.WriteFile(file, []byte{}, 0644))

	a := adapter.Adapter{
		ID:     "test",
		Detect: []adapter.DetectRule{{Path: file}},
	}
	assert.True(t, adapter.Detect(a))
}

func TestDetect_PathRule_Missing(t *testing.T) {
	a := adapter.Adapter{
		ID:     "test",
		Detect: []adapter.DetectRule{{Path: "/sap-devs-nonexistent-path-xyz/tool"}},
	}
	assert.False(t, adapter.Detect(a))
}

func TestDetect_CommandRule_Success(t *testing.T) {
	// "go version" is always available in CI and on dev machines with Go installed
	a := adapter.Adapter{
		ID:     "test",
		Detect: []adapter.DetectRule{{Command: "go version"}},
	}
	assert.True(t, adapter.Detect(a))
}

func TestDetect_CommandRule_Fail(t *testing.T) {
	a := adapter.Adapter{
		ID:     "test",
		Detect: []adapter.DetectRule{{Command: "sap-devs-nonexistent-binary-xyz"}},
	}
	assert.False(t, adapter.Detect(a))
}

func TestDetect_AnyPassesReturnsTrue(t *testing.T) {
	dir := t.TempDir()
	file := filepath.Join(dir, "exists")
	require.NoError(t, os.WriteFile(file, []byte{}, 0644))

	a := adapter.Adapter{
		ID: "test",
		Detect: []adapter.DetectRule{
			{Command: "sap-devs-nonexistent-binary-xyz"}, // fails
			{Path: file},                                  // passes
		},
	}
	assert.True(t, adapter.Detect(a))
}

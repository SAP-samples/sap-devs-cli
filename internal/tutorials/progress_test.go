package tutorials_test

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/SAP-samples/sap-devs-cli/internal/tutorials"
)

func TestProgress_NewTutorial(t *testing.T) {
	dir := t.TempDir()
	err := tutorials.UpdateProgress(dir, "test-tut", 1, 5, false)
	require.NoError(t, err)

	p, err := tutorials.GetProgress(dir, "test-tut")
	require.NoError(t, err)
	require.NotNil(t, p)
	assert.Equal(t, 1, p.CurrentStep)
	assert.Equal(t, 5, p.TotalSteps)
	assert.Empty(t, p.CompletedSteps)
	assert.False(t, p.StartedAt.IsZero())
}

func TestProgress_MarkStepDone(t *testing.T) {
	dir := t.TempDir()
	require.NoError(t, tutorials.UpdateProgress(dir, "test-tut", 1, 3, true))
	require.NoError(t, tutorials.UpdateProgress(dir, "test-tut", 2, 3, false))

	p, err := tutorials.GetProgress(dir, "test-tut")
	require.NoError(t, err)
	assert.Equal(t, 2, p.CurrentStep)
	assert.Equal(t, []int{1}, p.CompletedSteps)
	assert.Nil(t, p.CompletedAt)
}

func TestProgress_AllStepsDone(t *testing.T) {
	dir := t.TempDir()
	require.NoError(t, tutorials.UpdateProgress(dir, "test-tut", 1, 2, true))
	require.NoError(t, tutorials.UpdateProgress(dir, "test-tut", 2, 2, true))

	p, err := tutorials.GetProgress(dir, "test-tut")
	require.NoError(t, err)
	assert.NotNil(t, p.CompletedAt)
	assert.True(t, p.CompletedAt.Before(time.Now().Add(time.Second)))
}

func TestProgress_NoDuplicateCompletedSteps(t *testing.T) {
	dir := t.TempDir()
	require.NoError(t, tutorials.UpdateProgress(dir, "test-tut", 1, 3, true))
	require.NoError(t, tutorials.UpdateProgress(dir, "test-tut", 1, 3, true))

	p, err := tutorials.GetProgress(dir, "test-tut")
	require.NoError(t, err)
	assert.Equal(t, []int{1}, p.CompletedSteps)
}

func TestProgress_NotStarted(t *testing.T) {
	dir := t.TempDir()
	p, err := tutorials.GetProgress(dir, "nonexistent")
	require.NoError(t, err)
	assert.Nil(t, p)
}

func TestProgress_LoadAll(t *testing.T) {
	dir := t.TempDir()
	require.NoError(t, tutorials.UpdateProgress(dir, "tut-a", 1, 3, false))
	require.NoError(t, tutorials.UpdateProgress(dir, "tut-b", 2, 5, false))

	all, err := tutorials.LoadProgress(dir)
	require.NoError(t, err)
	assert.Len(t, all, 2)
}

func TestMergeCompletedSteps_NewTutorial(t *testing.T) {
	dir := t.TempDir()
	p, err := tutorials.MergeCompletedSteps(dir, "test-tut", []int{1, 2}, 3, 5)
	require.NoError(t, err)
	assert.Equal(t, []int{1, 2}, p.CompletedSteps)
	assert.Equal(t, 3, p.CurrentStep)
	assert.Equal(t, 5, p.TotalSteps)
	assert.Nil(t, p.CompletedAt)
}

func TestMergeCompletedSteps_MergeIdempotent(t *testing.T) {
	dir := t.TempDir()
	_, err := tutorials.MergeCompletedSteps(dir, "test-tut", []int{1}, 2, 5)
	require.NoError(t, err)
	p, err := tutorials.MergeCompletedSteps(dir, "test-tut", []int{1, 2}, 3, 5)
	require.NoError(t, err)
	assert.Equal(t, []int{1, 2}, p.CompletedSteps)
	assert.Equal(t, 3, p.CurrentStep)
}

func TestMergeCompletedSteps_Completion(t *testing.T) {
	dir := t.TempDir()
	p, err := tutorials.MergeCompletedSteps(dir, "test-tut", []int{1, 2, 3}, 0, 3)
	require.NoError(t, err)
	assert.NotNil(t, p.CompletedAt)
}

func TestMergeCompletedSteps_OutOfRange(t *testing.T) {
	dir := t.TempDir()
	_, err := tutorials.MergeCompletedSteps(dir, "test-tut", []int{0, 6}, 1, 5)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "out of range")
}

func TestMergeCompletedSteps_DefaultCurrentStep(t *testing.T) {
	dir := t.TempDir()
	p, err := tutorials.MergeCompletedSteps(dir, "test-tut", []int{3, 1, 2}, 0, 5)
	require.NoError(t, err)
	assert.Equal(t, 4, p.CurrentStep)
}

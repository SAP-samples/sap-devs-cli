package config_test

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.tools.sap/developer-relations/sap-devs-cli/internal/config"
)

func TestLoad_DefaultsWhenNoFile(t *testing.T) {
	dir := t.TempDir()
	cfg, err := config.Load(dir)
	require.NoError(t, err)
	assert.Equal(t, 24*time.Hour, cfg.Sync.Tips)
	assert.Equal(t, 168*time.Hour, cfg.Sync.Resources)
	assert.False(t, cfg.Sync.Disabled)
}

func TestLoad_ReadsExistingFile(t *testing.T) {
	dir := t.TempDir()
	yaml := `company_repo: "https://github.com/myco/sap-content"`
	require.NoError(t, os.WriteFile(filepath.Join(dir, "config.yaml"), []byte(yaml), 0600))

	cfg, err := config.Load(dir)
	require.NoError(t, err)
	assert.Equal(t, "https://github.com/myco/sap-content", cfg.CompanyRepo)
}

func TestSaveAndLoad_RoundTrip(t *testing.T) {
	dir := t.TempDir()
	cfg := config.Default()
	cfg.CompanyRepo = "https://example.com/repo"
	require.NoError(t, cfg.Save(dir))

	loaded, err := config.Load(dir)
	require.NoError(t, err)
	assert.Equal(t, cfg.CompanyRepo, loaded.CompanyRepo)
}

func TestLoadProfile_DefaultsWhenNoFile(t *testing.T) {
	dir := t.TempDir()
	p, err := config.LoadProfile(dir)
	require.NoError(t, err)
	assert.Empty(t, p.ID)
}

func TestSaveAndLoadProfile_RoundTrip(t *testing.T) {
	dir := t.TempDir()
	p := &config.Profile{ID: "cap-developer"}
	require.NoError(t, config.SaveProfile(dir, p))

	loaded, err := config.LoadProfile(dir)
	require.NoError(t, err)
	assert.Equal(t, "cap-developer", loaded.ID)
}

package config_test

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/SAP-samples/sap-devs-cli/internal/config"
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

func TestConfigLanguageRoundTrip(t *testing.T) {
	dir := t.TempDir()
	cfg := config.Default()
	cfg.Language = "de"
	require.NoError(t, cfg.Save(dir))

	loaded, err := config.Load(dir)
	require.NoError(t, err)
	assert.Equal(t, "de", loaded.Language)
}

func TestConfigLanguageOmitempty(t *testing.T) {
	dir := t.TempDir()
	cfg := config.Default() // Language is ""
	require.NoError(t, cfg.Save(dir))

	data, err := os.ReadFile(filepath.Join(dir, "config.yaml"))
	require.NoError(t, err)
	assert.NotContains(t, string(data), "language",
		"empty Language should not appear in YAML output")
}

func TestLocation_RoundTrip(t *testing.T) {
	dir := t.TempDir()
	cfg := config.Default()
	cfg.Location = "Hamburg, Germany"
	require.NoError(t, cfg.Save(dir))

	loaded, err := config.Load(dir)
	require.NoError(t, err)
	assert.Equal(t, "Hamburg, Germany", loaded.Location)
}

func TestLocation_Omitempty(t *testing.T) {
	dir := t.TempDir()
	cfg := config.Default() // Location is ""
	require.NoError(t, cfg.Save(dir))

	data, err := os.ReadFile(filepath.Join(dir, "config.yaml"))
	require.NoError(t, err)
	assert.NotContains(t, string(data), "location",
		"empty Location should not appear in YAML output")
}

func TestTipRotation_DefaultIsEmpty(t *testing.T) {
	// Default() leaves Rotation as "" — tipSeed treats "" as "daily" at runtime
	cfg := config.Default()
	assert.Equal(t, "", cfg.Tip.Rotation)
}

func TestTipRotation_RoundTrip(t *testing.T) {
	dir := t.TempDir()
	cfg := config.Default()
	cfg.Tip.Rotation = "hourly"
	require.NoError(t, cfg.Save(dir))

	loaded, err := config.Load(dir)
	require.NoError(t, err)
	assert.Equal(t, "hourly", loaded.Tip.Rotation)
}

func TestTipRotation_Omitempty(t *testing.T) {
	// When Rotation is "" (zero value), the "tip" block must not appear in YAML
	dir := t.TempDir()
	cfg := config.Default() // Rotation is "" — zero value
	require.NoError(t, cfg.Save(dir))

	data, err := os.ReadFile(filepath.Join(dir, "config.yaml"))
	require.NoError(t, err)
	assert.NotContains(t, string(data), "\ntip:",
		"empty TipConfig should not appear in YAML output")
}

func TestTipRotation_MissingKeyLoadsEmpty(t *testing.T) {
	// Config files without a "tip" block load with Rotation == ""
	dir := t.TempDir()
	yaml := `language: "en"`
	require.NoError(t, os.WriteFile(filepath.Join(dir, "config.yaml"), []byte(yaml), 0600))

	cfg, err := config.Load(dir)
	require.NoError(t, err)
	assert.Equal(t, "", cfg.Tip.Rotation)
}

func TestServiceConfig_Defaults(t *testing.T) {
	dir := t.TempDir()
	cfg, err := config.Load(dir)
	require.NoError(t, err)
	assert.Equal(t, 6*time.Hour, cfg.Service.Interval)
}

func TestServiceConfig_RoundTrip(t *testing.T) {
	dir := t.TempDir()
	cfg := config.Default()
	cfg.Service.Interval = 12 * time.Hour
	require.NoError(t, cfg.Save(dir))

	loaded, err := config.Load(dir)
	require.NoError(t, err)
	assert.Equal(t, 12*time.Hour, loaded.Service.Interval)
}

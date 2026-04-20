package cli

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/spf13/viper"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestConfigSearchPaths_XDG(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", "/tmp/odi-xdg")
	t.Setenv("HOME", "/tmp/odi-home")
	paths := configSearchPaths()
	require.Contains(t, paths, filepath.Join("/tmp/odi-xdg", "odi"))
	require.Contains(t, paths, filepath.Join("/tmp/odi-home", ".config", "odi"))
	require.Contains(t, paths, "/etc/odi")
}

func TestLoadConfigFile_XDGDiscovery(t *testing.T) {
	defer viper.Reset()
	viper.Reset()

	xdg := t.TempDir()
	cfgDir := filepath.Join(xdg, "odi")
	require.NoError(t, os.MkdirAll(cfgDir, 0o755))
	cfgPath := filepath.Join(cfgDir, "config.yaml")
	require.NoError(t, os.WriteFile(cfgPath, []byte("backend: remote\nbackend-url: https://odi.example.com\n"), 0o644))

	t.Setenv("XDG_CONFIG_HOME", xdg)

	require.NoError(t, loadConfigFile(""))
	assert.Equal(t, "remote", viper.GetString("backend"))
	assert.Equal(t, "https://odi.example.com", viper.GetString("backend-url"))
}

func TestLoadConfigFile_ExplicitPath(t *testing.T) {
	defer viper.Reset()
	viper.Reset()

	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "custom.yaml")
	require.NoError(t, os.WriteFile(cfgPath, []byte("scanner-name: my-scanner\n"), 0o644))

	require.NoError(t, loadConfigFile(cfgPath))
	assert.Equal(t, "my-scanner", viper.GetString("scanner-name"))
}

func TestLoadConfigFile_MissingExplicitPathErrors(t *testing.T) {
	defer viper.Reset()
	viper.Reset()

	err := loadConfigFile(filepath.Join(t.TempDir(), "does-not-exist.yaml"))
	assert.Error(t, err)
}

func TestLoadConfigFile_NoConfigIsFine(t *testing.T) {
	defer viper.Reset()
	viper.Reset()

	// Point to a directory with no config — should not error.
	empty := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", empty)
	t.Setenv("HOME", empty)
	require.NoError(t, loadConfigFile(""))
}

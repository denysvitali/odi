package cli

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/viper"
)

// configSearchPaths returns the ordered list of directories viper should
// search for the config file, honouring XDG_CONFIG_HOME.
func configSearchPaths() []string {
	var paths []string
	if xdg := os.Getenv("XDG_CONFIG_HOME"); xdg != "" {
		paths = append(paths, filepath.Join(xdg, "odi"))
	}
	if home, err := os.UserHomeDir(); err == nil {
		paths = append(paths, filepath.Join(home, ".config", "odi"))
	}
	paths = append(paths, "/etc/odi")
	return paths
}

// loadConfigFile wires viper to read an optional config file. If explicitPath
// is set, only that file is loaded and a missing file is an error. Otherwise
// the XDG search paths are tried and a missing file is silently ignored.
func loadConfigFile(explicitPath string) error {
	if explicitPath != "" {
		viper.SetConfigFile(explicitPath)
		if err := viper.ReadInConfig(); err != nil {
			return fmt.Errorf("read config %q: %w", explicitPath, err)
		}
		return nil
	}

	viper.SetConfigName("config")
	viper.SetConfigType("yaml")
	for _, p := range configSearchPaths() {
		viper.AddConfigPath(p)
	}
	if err := viper.ReadInConfig(); err != nil {
		var notFound viper.ConfigFileNotFoundError
		if errors.As(err, &notFound) {
			return nil
		}
		return fmt.Errorf("read config: %w", err)
	}
	return nil
}

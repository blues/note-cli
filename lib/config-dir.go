// Copyright 2026 Blues Inc. All rights reserved.
// Use of this source code is governed by licenses granted by the copyright holder.

package lib

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
)

// ConfigDir returns the directory holding the configuration, creating it if needed.
// It is "note" within $XDG_CONFIG_HOME, or ~/.config when that isn't set, and within
// %AppData% on Windows.
func ConfigDir() (string, error) {
	base, err := configBaseDir()
	if err != nil {
		return "", fmt.Errorf("can't find the configuration directory: %w", err)
	}
	path := filepath.Join(base, "note")
	if err := os.MkdirAll(path, 0700); err != nil {
		return "", fmt.Errorf("can't create the configuration directory: %w", err)
	}
	return path, nil
}

func configBaseDir() (string, error) {
	if runtime.GOOS == "windows" {
		return os.UserConfigDir()
	}
	// A relative XDG_CONFIG_HOME is invalid and is ignored, as the XDG spec requires
	if dir := os.Getenv("XDG_CONFIG_HOME"); filepath.IsAbs(dir) {
		return dir, nil
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".config"), nil
}

// Get the pathname of config settings
func defaultConfigSettingsPath() (string, error) {
	dir, err := ConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "config.json"), nil
}

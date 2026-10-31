// Copyright 2026 Blues Inc. All rights reserved.
// Use of this source code is governed by licenses granted by the copyright holder.

package lib

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/blues/note-go/note"
)

var configSettingsPath = defaultConfigSettingsPath

// withConfigLock serializes credentials across processes. The operating system
// releases the lock when the process exits, including after a crash.
func withConfigLock(ctx context.Context, fn func() error) error {
	ctx, cancel := context.WithTimeout(ctx, 45*time.Second)
	defer cancel()
	path, err := configSettingsPath()
	if err != nil {
		return err
	}
	file, err := os.OpenFile(path+".lock", os.O_CREATE|os.O_RDWR, 0600)
	if err != nil {
		return fmt.Errorf("locking configuration: %w", err)
	}
	defer file.Close()
	for {
		locked, err := tryConfigLock(file)
		if err != nil {
			return fmt.Errorf("locking configuration: %w", err)
		}
		if locked {
			return fn()
		}
		select {
		case <-ctx.Done():
			return fmt.Errorf("waiting for configuration lock: %w", ctx.Err())
		case <-time.After(25 * time.Millisecond):
		}
	}
}

// writeConfigJSON replaces a complete file atomically, so readers never see half
// of a rotated token pair. The caller holds the configuration lock.
func writeConfigJSON(contents []byte) error {
	path, err := configSettingsPath()
	if err != nil {
		return err
	}
	file, err := os.CreateTemp(filepath.Dir(path), ".config-*")
	if err != nil {
		return fmt.Errorf("saving configuration: %w", err)
	}
	defer os.Remove(file.Name())
	if _, err = file.Write(contents); err == nil {
		err = file.Sync()
	}
	closeErr := file.Close()
	if err == nil {
		err = closeErr
	}
	if err == nil {
		err = os.Rename(file.Name(), path)
	}
	if err != nil {
		return fmt.Errorf("saving configuration: %w", err)
	}
	return nil
}

// withStoredCredentials rereads the latest saved state while locked and commits
// only the credential change. Session-only --hub and other flags aren't saved.
func (config *ConfigSettings) withStoredCredentials(ctx context.Context, update func(*ConfigSettings) (bool, error)) error {
	return withConfigLock(ctx, func() error {
		stored, err := readConfigFromFile()
		if err != nil {
			return err
		}
		if stored == nil {
			return fmt.Errorf("saved configuration is missing; run 'notehub login'")
		}
		changed, err := update(stored)
		if err != nil {
			return err
		}
		if changed {
			contents, err := note.JSONMarshalIndent(stored, "", "    ")
			if err != nil {
				return err
			}
			if err := writeConfigJSON(contents); err != nil {
				return err
			}
		}
		config.HubCreds = stored.HubCreds
		return nil
	})
}

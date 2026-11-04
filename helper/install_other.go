//go:build !windows

package main

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
)

// On macOS and Linux Chrome looks for the registration file in a folder of the user, one folder per browser.
type browserDir struct{ name, dir string }

func browserDirs() []browserDir {
	home, err := os.UserHomeDir()
	if err != nil {
		return nil
	}
	if runtime.GOOS == "darwin" {
		base := filepath.Join(home, "Library", "Application Support")
		return []browserDir{
			{"Google Chrome", filepath.Join(base, "Google", "Chrome", "NativeMessagingHosts")},
			{"Microsoft Edge", filepath.Join(base, "Microsoft Edge", "NativeMessagingHosts")},
			{"Brave", filepath.Join(base, "BraveSoftware", "Brave-Browser", "NativeMessagingHosts")},
			{"Chromium", filepath.Join(base, "Chromium", "NativeMessagingHosts")},
		}
	}
	config := os.Getenv("XDG_CONFIG_HOME")
	if config == "" {
		config = filepath.Join(home, ".config")
	}
	return []browserDir{
		{"Google Chrome", filepath.Join(config, "google-chrome", "NativeMessagingHosts")},
		{"Microsoft Edge", filepath.Join(config, "microsoft-edge", "NativeMessagingHosts")},
		{"Brave", filepath.Join(config, "BraveSoftware", "Brave-Browser", "NativeMessagingHosts")},
		{"Chromium", filepath.Join(config, "chromium", "NativeMessagingHosts")},
	}
}

func registerHost(_ string, manifest []byte) ([]string, error) {
	var done []string
	for _, b := range browserDirs() {
		if err := os.MkdirAll(b.dir, 0o755); err != nil {
			return done, fmt.Errorf("the registration for %s failed: %w", b.name, err)
		}
		if err := os.WriteFile(filepath.Join(b.dir, hostName+".json"), manifest, 0o644); err != nil {
			return done, fmt.Errorf("the registration for %s failed: %w", b.name, err)
		}
		done = append(done, b.name)
	}
	return done, nil
}

func unregisterHost() error {
	for _, b := range browserDirs() {
		if err := os.Remove(filepath.Join(b.dir, hostName+".json")); err != nil && !errors.Is(err, os.ErrNotExist) {
			return fmt.Errorf("the registration for %s could not be removed: %w", b.name, err)
		}
	}
	return nil
}

// registrations are the registration files that exist: for each browser, the file it reads.
func registrations() []registration {
	var found []registration
	for _, b := range browserDirs() {
		file := filepath.Join(b.dir, hostName+".json")
		if _, err := os.Stat(file); err == nil {
			found = append(found, registration{Browser: b.name, Manifest: file})
		}
	}
	return found
}

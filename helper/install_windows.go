//go:build windows

package main

import (
	"errors"
	"fmt"

	"golang.org/x/sys/windows/registry"
)

// Chrome on Windows finds a helper through a registry value of the current user. Edge, Brave and Chromium read the same
// kind of value under their own names, so one setup serves all of them.
type browserKey struct{ name, key string }

var browserKeys = []browserKey{
	{"Google Chrome", `Software\Google\Chrome\NativeMessagingHosts\` + hostName},
	{"Microsoft Edge", `Software\Microsoft\Edge\NativeMessagingHosts\` + hostName},
	{"Brave", `Software\BraveSoftware\Brave-Browser\NativeMessagingHosts\` + hostName},
	{"Chromium", `Software\Chromium\NativeMessagingHosts\` + hostName},
}

func registerHost(manifestPath string, _ []byte) ([]string, error) {
	var done []string
	for _, b := range browserKeys {
		key, _, err := registry.CreateKey(registry.CURRENT_USER, b.key, registry.SET_VALUE)
		if err != nil {
			return done, fmt.Errorf("the registration for %s failed: %w", b.name, err)
		}
		err = key.SetStringValue("", manifestPath)
		key.Close()
		if err != nil {
			return done, fmt.Errorf("the registration for %s failed: %w", b.name, err)
		}
		done = append(done, b.name)
	}
	return done, nil
}

func unregisterHost() error {
	for _, b := range browserKeys {
		if err := registry.DeleteKey(registry.CURRENT_USER, b.key); err != nil && !errors.Is(err, registry.ErrNotExist) {
			return fmt.Errorf("the registration for %s could not be removed: %w", b.name, err)
		}
	}
	return nil
}

// registrations are the registry values that exist: for each browser, the file it reads.
func registrations() []registration {
	var found []registration
	for _, b := range browserKeys {
		key, err := registry.OpenKey(registry.CURRENT_USER, b.key, registry.QUERY_VALUE)
		if err != nil {
			continue
		}
		value, _, err := key.GetStringValue("")
		key.Close()
		if err == nil {
			found = append(found, registration{Browser: b.name, Manifest: value})
		}
	}
	return found
}

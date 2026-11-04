//go:build windows

package main

import (
	"errors"
	"strconv"

	"golang.org/x/sys/windows/registry"
)

// readWinSCPRegistry reads the sessions that WinSCP keeps in the registry of the current user (its default place; the
// INI file is read in sites.go). Only the values named in winscpFields are asked for: never the password or a key file.
// found says whether WinSCP has a session list there at all.
func readWinSCPRegistry(root string) (sessions []winscpSession, found bool, err error) {
	key, err := registry.OpenKey(registry.CURRENT_USER, root, registry.ENUMERATE_SUB_KEYS)
	if err != nil {
		if errors.Is(err, registry.ErrNotExist) {
			return nil, false, nil
		}
		return nil, false, err
	}
	defer key.Close()
	names, err := key.ReadSubKeyNames(-1)
	if err != nil {
		return nil, true, err
	}
	for _, name := range names {
		if len(sessions) >= maxSitesPerSource*2 {
			break
		}
		sub, err := registry.OpenKey(key, name, registry.QUERY_VALUE)
		if err != nil {
			continue
		}
		values := map[string]string{}
		for _, field := range winscpFields {
			if text, _, err := sub.GetStringValue(field); err == nil {
				values[field] = text
			} else if number, _, err := sub.GetIntegerValue(field); err == nil {
				values[field] = strconv.FormatUint(number, 10)
			}
		}
		sub.Close()
		sessions = append(sessions, winscpSession{Name: name, Values: values})
	}
	return sessions, true, nil
}

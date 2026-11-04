//go:build windows

package main

import (
	"fmt"
	"testing"
	"time"

	"golang.org/x/sys/windows/registry"
)

// useTestRegistry points the registration at a registry branch that only this test uses, and removes it afterwards.
func useTestRegistry(t *testing.T) {
	t.Helper()
	root := fmt.Sprintf(`Software\RocuoHelperTest-%d`, time.Now().UnixNano())
	saved := browserKeys
	browserKeys = []browserKey{{"Test browser", root + `\NativeMessagingHosts\` + hostName}}
	t.Cleanup(func() {
		browserKeys = saved
		_ = registry.DeleteKey(registry.CURRENT_USER, root+`\NativeMessagingHosts\`+hostName)
		_ = registry.DeleteKey(registry.CURRENT_USER, root+`\NativeMessagingHosts`)
		_ = registry.DeleteKey(registry.CURRENT_USER, root)
	})
}

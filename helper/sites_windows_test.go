//go:build windows

package main

import (
	"reflect"
	"testing"

	"golang.org/x/sys/windows/registry"
)

// WinSCP keeps its sessions in the registry unless the person chose an INI file. The test builds a sample under its own
// key of the current user (never WinSCP's real one) and removes it afterwards.
func TestWinSCPRegistry(t *testing.T) {
	root := `Software\RocuoHelperTest\WinSCP\Sessions`
	t.Cleanup(func() {
		for _, name := range []string{`Registry%20server`, `Dav`} {
			_ = registry.DeleteKey(registry.CURRENT_USER, root+`\`+name)
		}
		_ = registry.DeleteKey(registry.CURRENT_USER, root)
		_ = registry.DeleteKey(registry.CURRENT_USER, `Software\RocuoHelperTest\WinSCP`)
		_ = registry.DeleteKey(registry.CURRENT_USER, `Software\RocuoHelperTest`)
	})

	createSession := func(name string, texts map[string]string, numbers map[string]uint32) {
		key, _, err := registry.CreateKey(registry.CURRENT_USER, root+`\`+name, registry.SET_VALUE)
		if err != nil {
			t.Fatal(err)
		}
		defer key.Close()
		for k, v := range texts {
			if err := key.SetStringValue(k, v); err != nil {
				t.Fatal(err)
			}
		}
		for k, v := range numbers {
			if err := key.SetDWordValue(k, v); err != nil {
				t.Fatal(err)
			}
		}
	}
	createSession(`Registry%20server`,
		map[string]string{"HostName": "reg.example.com", "UserName": "frank", "Password": "SECRETHASH", "RemoteDirectory": "/data"},
		map[string]uint32{"PortNumber": 2200, "FSProtocol": 2})
	createSession(`Dav`, map[string]string{"HostName": "dav.example.com"}, map[string]uint32{"FSProtocol": 6})

	out := readWinSCP(siteLocations{appData: t.TempDir()}, root)
	if !out.Found || out.Error != "" {
		t.Fatalf("found = %v, error = %q", out.Found, out.Error)
	}
	want := []importedSite{{Name: "Registry server", Kind: "sftp", Host: "reg.example.com", Port: 2200, User: "frank", Path: "/data"}}
	if !reflect.DeepEqual(out.Sites, want) || out.Skipped != 1 {
		t.Fatalf("sites = %+v (skipped %d), want %+v and 1", out.Sites, out.Skipped, want)
	}
	assertNothingSecret(t, out)
}

package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestValidExtensionID(t *testing.T) {
	good := []string{"pckgpblcplbjffapabndpkhnhkioihjd", "abcdefghijklmnopabcdefghijklmnop"}
	bad := []string{"", "short", "PCKGPBLCPLBJFFAPABNDPKHNHKIOIHJD", "pckgpblcplbjffapabndpkhnhkioihjz", "pckgpblcplbjffapabndpkhnhkioihjdd"}
	for _, id := range good {
		if !validExtensionID(id) {
			t.Errorf("%q should be valid", id)
		}
	}
	for _, id := range bad {
		if validExtensionID(id) {
			t.Errorf("%q should not be valid", id)
		}
	}
}

func TestManifestAllowsOnlyListedExtensions(t *testing.T) {
	raw, err := buildManifest(`C:\x\rocuo-files-helper.exe`, []string{"abcdefghijklmnopabcdefghijklmnop", "abcdefghijklmnopabcdefghijklmnop", ""})
	if err != nil {
		t.Fatal(err)
	}
	var m struct {
		Name           string   `json:"name"`
		Path           string   `json:"path"`
		Type           string   `json:"type"`
		AllowedOrigins []string `json:"allowed_origins"`
	}
	if err := json.Unmarshal(raw, &m); err != nil {
		t.Fatal(err)
	}
	if m.Name != "com.rocuo.remote_files_helper" || m.Type != "stdio" || m.Path != `C:\x\rocuo-files-helper.exe` {
		t.Fatalf("manifest = %+v", m)
	}
	want := []string{"chrome-extension://pckgpblcplbjffapabndpkhnhkioihjd/", "chrome-extension://abcdefghijklmnopabcdefghijklmnop/"}
	if strings.Join(m.AllowedOrigins, "|") != strings.Join(want, "|") {
		t.Fatalf("allowed origins = %v, want %v (the published one first, no doubles, no empty)", m.AllowedOrigins, want)
	}
}

func TestInstallCopiesRegistersAndUninstalls(t *testing.T) {
	useTestRegistry(t)
	dir := filepath.Join(t.TempDir(), "helper")

	if _, err := install(installOptions{Dir: dir, ExtensionIDs: []string{"not-an-id"}}); err == nil {
		t.Fatal("a bad extension address must be refused")
	}

	res, err := install(installOptions{Dir: dir, ExtensionIDs: []string{"abcdefghijklmnopabcdefghijklmnop"}})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(res.Exe); err != nil {
		t.Fatalf("the program was not copied: %v", err)
	}
	if filepath.Dir(res.Exe) != dir {
		t.Fatalf("copied to %s, want %s", res.Exe, dir)
	}
	raw, err := os.ReadFile(res.Manifest)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(raw), "abcdefghijklmnopabcdefghijklmnop") || !strings.Contains(string(raw), "pckgpblcplbjffapabndpkhnhkioihjd") {
		t.Fatalf("manifest lacks the extension addresses: %s", raw)
	}
	if len(res.Registered) == 0 || len(registeredHosts()) == 0 {
		t.Fatalf("nothing was registered: %v / %v", res.Registered, registeredHosts())
	}

	// A second setup over the first (an update) works too.
	if _, err := install(installOptions{Dir: dir}); err != nil {
		t.Fatalf("setup over an earlier setup: %v", err)
	}

	if err := uninstall(installOptions{Dir: dir}); err != nil {
		t.Fatal(err)
	}
	if len(registeredHosts()) != 0 {
		t.Fatalf("still registered after uninstall: %v", registeredHosts())
	}
	if _, err := os.Stat(res.Manifest); err == nil {
		t.Fatal("the registration file is still there after uninstall")
	}
}

const (
	idA = "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
	idB = "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb"
	idC = "cccccccccccccccccccccccccccccccc"
	// the published address (the Chrome Web Store)
	idStore = "pckgpblcplbjffapabndpkhnhkioihjd"
)

func TestInstallKeepsTheAddressesOfAnEarlierSetup(t *testing.T) {
	useTestRegistry(t)
	dir := filepath.Join(t.TempDir(), "helper")

	first, err := install(installOptions{Dir: dir, ExtensionIDs: []string{idA}})
	if err != nil {
		t.Fatal(err)
	}
	if strings.Join(first.Extensions, ",") != idStore+","+idA {
		t.Fatalf("first setup allows %v", first.Extensions)
	}

	// Another copy of the extension is added: the first one stays.
	second, err := install(installOptions{Dir: dir, ExtensionIDs: []string{idB}})
	if err != nil {
		t.Fatal(err)
	}
	if strings.Join(second.Extensions, ",") != idStore+","+idA+","+idB {
		t.Fatalf("second setup allows %v, want the published one and both added ones", second.Extensions)
	}

	// An update (the double-click on a new download, no options) must not forget them either.
	third, err := install(installOptions{Dir: dir})
	if err != nil {
		t.Fatal(err)
	}
	if strings.Join(third.Extensions, ",") != idStore+","+idA+","+idB {
		t.Fatalf("an update allows %v, it must keep what was added", third.Extensions)
	}
	if got := earlierExtensionIDs(third.Manifest); strings.Join(got, ",") != idStore+","+idA+","+idB {
		t.Fatalf("the file on disk says %v", got)
	}

	// The same address twice is one address.
	again, err := install(installOptions{Dir: dir, ExtensionIDs: []string{idA}})
	if err != nil {
		t.Fatal(err)
	}
	if len(again.Extensions) != 3 {
		t.Fatalf("the same address twice: %v", again.Extensions)
	}

	// "Start again" forgets them and keeps only the published one and what is given now.
	reset, err := install(installOptions{Dir: dir, ExtensionIDs: []string{idC}, ResetExtensions: true})
	if err != nil {
		t.Fatal(err)
	}
	if strings.Join(reset.Extensions, ",") != idStore+","+idC {
		t.Fatalf("after a reset the helper allows %v", reset.Extensions)
	}
	bare, err := install(installOptions{Dir: dir, ResetExtensions: true})
	if err != nil {
		t.Fatal(err)
	}
	if strings.Join(bare.Extensions, ",") != idStore {
		t.Fatalf("a reset with nothing added allows %v, want only the published one", bare.Extensions)
	}
}

func TestEarlierExtensionIDsTrustsOnlyAddressesChromeAccepts(t *testing.T) {
	dir := t.TempDir()
	if ids := earlierExtensionIDs(filepath.Join(dir, "none.json")); len(ids) != 0 {
		t.Fatalf("a missing file gave %v", ids)
	}
	damaged := filepath.Join(dir, "damaged.json")
	if err := os.WriteFile(damaged, []byte("not json {"), 0o644); err != nil {
		t.Fatal(err)
	}
	if ids := earlierExtensionIDs(damaged); len(ids) != 0 {
		t.Fatalf("a damaged file gave %v", ids)
	}
	mixed := filepath.Join(dir, "mixed.json")
	body := `{"allowed_origins": ["https://example.com/", "chrome-extension://short/", "chrome-extension://` + idA + `", "chrome-extension://` + idB + `/", "moz-extension://` + idC + `/"]}`
	if err := os.WriteFile(mixed, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	if ids := earlierExtensionIDs(mixed); strings.Join(ids, ",") != idB {
		t.Fatalf("kept %v, want only the one valid chrome-extension address with its slash", ids)
	}
}

func TestIDFromOrigin(t *testing.T) {
	if id, ok := idFromOrigin("chrome-extension://" + idA + "/"); !ok || id != idA {
		t.Fatalf("a good address was refused: %q %v", id, ok)
	}
	for _, bad := range []string{"", "chrome-extension://" + idA, "chrome-extension://" + idA + "/x/", "chrome-extension://UPPERCASEUPPERCASEUPPERCASEUPPER/", "http://" + idA + "/"} {
		if id, ok := idFromOrigin(bad); ok {
			t.Errorf("%q should not be an address, got %q", bad, id)
		}
	}
}

func TestStatusReportShowsWhatChromeIsToldAndWhoMayUseIt(t *testing.T) {
	useTestRegistry(t)
	if report := statusReport(); !strings.Contains(report, "nothing yet") {
		t.Fatalf("with nothing registered the report says:\n%s", report)
	}

	dir := filepath.Join(t.TempDir(), "helper")
	res, err := install(installOptions{Dir: dir, ExtensionIDs: []string{idA}})
	if err != nil {
		t.Fatal(err)
	}
	report := statusReport()
	for _, want := range []string{
		"Registered for:",
		"Chrome starts: " + res.Exe + " (found)",
		idStore + " (Chrome Web Store)",
		idA + " (added with --extension-id)",
	} {
		if !strings.Contains(report, want) {
			t.Errorf("the report lacks %q:\n%s", want, report)
		}
	}

	// The program that Chrome would start is gone: the report must say so (this is what a person needs to read).
	if err := os.Remove(res.Exe); err != nil {
		t.Fatal(err)
	}
	if report := statusReport(); !strings.Contains(report, "NOT FOUND") {
		t.Fatalf("a missing program is not reported:\n%s", report)
	}

	// The registration file is gone but Chrome is still pointed at it (on Windows the registry points at the file; on
	// the other systems each browser has a copy of it).
	for _, r := range registrations() {
		if err := os.Remove(r.Manifest); err != nil {
			t.Fatal(err)
		}
	}
	if report := statusReport(); !strings.Contains(report, "cannot be read") && !strings.Contains(report, "nothing yet") {
		t.Fatalf("a missing registration file is not reported:\n%s", report)
	}
}

func TestWhatInstallSays(t *testing.T) {
	text := installedText(&installResult{Dir: `C:\x`, Registered: []string{"Google Chrome", "Microsoft Edge"}, Extensions: []string{idStore, idA}})
	for _, want := range []string{
		`The helper is installed in C:\x`,
		"Google Chrome, Microsoft Edge",
		idStore + " (Chrome Web Store)",
		idA + " (added with --extension-id)",
		`press "Check again"`,
	} {
		if !strings.Contains(text, want) {
			t.Errorf("the message lacks %q:\n%s", want, text)
		}
	}
	if extra := extraExtensions([]string{idStore, idA, idB}); strings.Join(extra, ",") != idA+","+idB {
		t.Fatalf("extra extensions = %v", extra)
	}
	if extra := extraExtensions([]string{idStore}); len(extra) != 0 {
		t.Fatalf("the published address alone has no extra ones, got %v", extra)
	}
}

func TestInstallWithoutCopyRegistersTheRunningProgram(t *testing.T) {
	useTestRegistry(t)
	dir := t.TempDir()
	res, err := install(installOptions{Dir: dir, NoCopy: true})
	if err != nil {
		t.Fatal(err)
	}
	self, _ := os.Executable()
	self, _ = filepath.EvalSymlinks(self)
	if res.Exe != self {
		t.Fatalf("registered %s, want the running program %s", res.Exe, self)
	}
	if _, err := os.Stat(filepath.Join(dir, exeName())); err == nil {
		t.Fatal("--no-copy must not copy")
	}
}

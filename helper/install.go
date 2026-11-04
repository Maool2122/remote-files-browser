package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"
)

// Setup: the helper copies itself to a folder of the user, writes the small file that tells Chrome where the helper is
// ("native messaging host manifest") and registers that file. Nothing needs administrator rights and nothing outside the
// user's own folders and registry branch is touched.

const hostName = "com.rocuo.remote_files_helper"

// The address of the extension on the Chrome Web Store. Chrome starts the helper only for extensions on this list.
var publishedExtensionIDs = []string{"pckgpblcplbjffapabndpkhnhkioihjd"}

type installOptions struct {
	ExtensionIDs    []string // added to the published one (for a copy of the extension loaded from a folder)
	ResetExtensions bool     // forget the addresses of an earlier setup: only the published one and ExtensionIDs stay
	Dir             string   // where to put the helper; empty: the usual place of the system
	NoCopy          bool     // register the running program where it is
}

type installResult struct {
	Dir        string
	Exe        string
	Manifest   string
	Registered []string
	Extensions []string // the addresses of the extensions that may use the helper now
}

func exeName() string {
	if runtime.GOOS == "windows" {
		return "rocuo-files-helper.exe"
	}
	return "rocuo-files-helper"
}

func defaultInstallDir() (string, error) {
	switch runtime.GOOS {
	case "windows":
		base := os.Getenv("LOCALAPPDATA")
		if base == "" {
			home, err := os.UserHomeDir()
			if err != nil {
				return "", err
			}
			base = filepath.Join(home, "AppData", "Local")
		}
		return filepath.Join(base, "Rocuo", "RemoteFilesHelper"), nil
	case "darwin":
		home, err := os.UserHomeDir()
		if err != nil {
			return "", err
		}
		return filepath.Join(home, "Library", "Application Support", "Rocuo", "RemoteFilesHelper"), nil
	default:
		base := os.Getenv("XDG_DATA_HOME")
		if base == "" {
			home, err := os.UserHomeDir()
			if err != nil {
				return "", err
			}
			base = filepath.Join(home, ".local", "share")
		}
		return filepath.Join(base, "rocuo", "remote-files-helper"), nil
	}
}

const originPrefix = "chrome-extension://"

// allowedIDs is the published address first, then the extra ones, each once and none empty.
func allowedIDs(extra []string) []string {
	seen := map[string]bool{}
	var ids []string
	for _, id := range append(append([]string{}, publishedExtensionIDs...), extra...) {
		id = strings.TrimSpace(id)
		if id == "" || seen[id] {
			continue
		}
		seen[id] = true
		ids = append(ids, id)
	}
	return ids
}

func allowedOrigins(extra []string) []string {
	var origins []string
	for _, id := range allowedIDs(extra) {
		origins = append(origins, originPrefix+id+"/")
	}
	return origins
}

// idFromOrigin takes the address out of "chrome-extension://ID/". It is false for anything else.
func idFromOrigin(origin string) (string, bool) {
	if !strings.HasPrefix(origin, originPrefix) || !strings.HasSuffix(origin, "/") {
		return "", false
	}
	id := strings.TrimSuffix(strings.TrimPrefix(origin, originPrefix), "/")
	return id, validExtensionID(id)
}

// registrationFile is what Chrome reads (the "native messaging host manifest").
type registrationFile struct {
	Name           string   `json:"name"`
	Path           string   `json:"path"`
	Type           string   `json:"type"`
	AllowedOrigins []string `json:"allowed_origins"`
}

func readRegistration(path string) (*registrationFile, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var file registrationFile
	if err := json.Unmarshal(raw, &file); err != nil {
		return nil, err
	}
	return &file, nil
}

// earlierExtensionIDs are the addresses an earlier setup in this folder allowed. An update, or the double-click on a new
// download, keeps them: a person who added their own copy of the extension does not have to do it again. Nothing is
// returned when there was no setup or its file is damaged.
func earlierExtensionIDs(manifestPath string) []string {
	file, err := readRegistration(manifestPath)
	if err != nil {
		return nil
	}
	var ids []string
	for _, origin := range file.AllowedOrigins {
		if id, ok := idFromOrigin(origin); ok {
			ids = append(ids, id)
		}
	}
	return ids
}

func validExtensionID(id string) bool {
	if len(id) != 32 {
		return false
	}
	for _, r := range id {
		if r < 'a' || r > 'p' {
			return false
		}
	}
	return true
}

func buildManifest(exe string, ids []string) ([]byte, error) {
	manifest := map[string]any{
		"name":            hostName,
		"description":     "Remote Files Browser helper: FTP, FTPS and SFTP for the Remote Files Browser extension",
		"path":            exe,
		"type":            "stdio",
		"allowed_origins": allowedOrigins(ids),
	}
	return json.MarshalIndent(manifest, "", "  ")
}

func copyFile(from, to string) error {
	src, err := os.Open(from)
	if err != nil {
		return err
	}
	defer src.Close()
	tmp := to + ".new"
	dst, err := os.OpenFile(tmp, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o755)
	if err != nil {
		return err
	}
	if _, err := io.Copy(dst, src); err != nil {
		dst.Close()
		os.Remove(tmp)
		return err
	}
	if err := dst.Close(); err != nil {
		os.Remove(tmp)
		return err
	}
	// A program that is running cannot be overwritten on Windows, but it can be renamed out of the way.
	if err := os.Rename(tmp, to); err != nil {
		old := fmt.Sprintf("%s.old-%d", to, time.Now().UnixNano())
		if rerr := os.Rename(to, old); rerr != nil {
			os.Remove(tmp)
			return err
		}
		if err := os.Rename(tmp, to); err != nil {
			os.Rename(old, to)
			os.Remove(tmp)
			return err
		}
	}
	return nil
}

// cleanOld removes the leftovers of earlier updates (best effort: a copy that is still running stays).
func cleanOld(dir string) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return
	}
	for _, e := range entries {
		if strings.Contains(e.Name(), ".old-") {
			os.Remove(filepath.Join(dir, e.Name()))
		}
	}
}

func sameFile(a, b string) bool {
	ia, err := os.Stat(a)
	if err != nil {
		return false
	}
	ib, err := os.Stat(b)
	if err != nil {
		return false
	}
	return os.SameFile(ia, ib)
}

func install(opts installOptions) (*installResult, error) {
	for _, id := range opts.ExtensionIDs {
		if !validExtensionID(id) {
			return nil, fmt.Errorf("%q is not an extension address (32 letters from a to p)", id)
		}
	}
	self, err := os.Executable()
	if err != nil {
		return nil, err
	}
	self, _ = filepath.EvalSymlinks(self)

	dir := opts.Dir
	if dir == "" {
		if dir, err = defaultInstallDir(); err != nil {
			return nil, err
		}
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, fmt.Errorf("the folder %s could not be made: %w", dir, err)
	}

	exe := self
	if !opts.NoCopy {
		exe = filepath.Join(dir, exeName())
		if !sameFile(self, exe) {
			if err := copyFile(self, exe); err != nil {
				return nil, fmt.Errorf("the helper could not be copied to %s: %w", dir, err)
			}
		}
		cleanOld(dir)
	}

	manifestPath := filepath.Join(dir, hostName+".json")
	ids := opts.ExtensionIDs
	if !opts.ResetExtensions {
		ids = append(earlierExtensionIDs(manifestPath), ids...)
	}
	manifest, err := buildManifest(exe, ids)
	if err != nil {
		return nil, err
	}
	if err := os.WriteFile(manifestPath, manifest, 0o644); err != nil {
		return nil, fmt.Errorf("the registration file could not be written: %w", err)
	}
	registered, err := registerHost(manifestPath, manifest)
	if err != nil {
		return nil, err
	}
	return &installResult{Dir: dir, Exe: exe, Manifest: manifestPath, Registered: registered, Extensions: allowedIDs(ids)}, nil
}

func uninstall(opts installOptions) error {
	dir := opts.Dir
	var err error
	if dir == "" {
		if dir, err = defaultInstallDir(); err != nil {
			return err
		}
	}
	if err := unregisterHost(); err != nil {
		return err
	}
	_ = os.Remove(filepath.Join(dir, hostName+".json"))
	exe := filepath.Join(dir, exeName())
	if err := os.Remove(exe); err != nil && !errors.Is(err, os.ErrNotExist) {
		// The program is running right now (Windows does not allow deleting it): the folder can be deleted by hand.
		_ = os.Rename(exe, exe+fmt.Sprintf(".old-%d", time.Now().UnixNano()))
	}
	cleanOld(dir)
	_ = os.Remove(dir) // only when it is empty
	return nil
}

// registration says where a browser finds the registration file.
type registration struct{ Browser, Manifest string }

// registeredHosts are the registrations that exist, one line each ("Google Chrome -> the file").
func registeredHosts() []string {
	var lines []string
	for _, r := range registrations() {
		lines = append(lines, fmt.Sprintf("%s -> %s", r.Browser, r.Manifest))
	}
	return lines
}

func describeExtension(id string) string {
	for _, published := range publishedExtensionIDs {
		if id == published {
			return id + " (Chrome Web Store)"
		}
	}
	return id + " (added with --extension-id)"
}

// statusReport says what is installed, as plain lines: what Chrome is told, whether the program it would start is there,
// and which extensions may use it. It is what to look at when the extension says the helper is not found or not allowed.
func statusReport() string {
	var b strings.Builder
	fmt.Fprintf(&b, "Remote Files Browser helper %s (%s/%s)\n", version, runtime.GOOS, runtime.GOARCH)
	if self, err := os.Executable(); err == nil {
		fmt.Fprintf(&b, "This program: %s\n", self)
	}
	regs := registrations()
	if len(regs) == 0 {
		b.WriteString("Registered for: nothing yet (open this program once, without arguments, to set it up)\n")
		return b.String()
	}
	b.WriteString("Registered for:\n")
	for _, r := range regs {
		fmt.Fprintf(&b, "  %s -> %s\n", r.Browser, r.Manifest)
	}
	seen := map[string]bool{}
	for _, r := range regs {
		if seen[r.Manifest] {
			continue
		}
		seen[r.Manifest] = true
		file, err := readRegistration(r.Manifest)
		if err != nil {
			fmt.Fprintf(&b, "The registration file %s cannot be read (%v). Set the helper up again.\n", r.Manifest, err)
			continue
		}
		found := "found"
		if _, err := os.Stat(file.Path); err != nil {
			found = "NOT FOUND: set the helper up again"
		}
		fmt.Fprintf(&b, "Chrome starts: %s (%s)\n", file.Path, found)
		b.WriteString("Extensions that may use it:\n")
		for _, origin := range file.AllowedOrigins {
			if id, ok := idFromOrigin(origin); ok {
				fmt.Fprintf(&b, "  %s\n", describeExtension(id))
			} else {
				fmt.Fprintf(&b, "  %s (not an extension address Chrome accepts)\n", origin)
			}
		}
	}
	return b.String()
}

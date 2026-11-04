package main

import (
	"bytes"
	"encoding/xml"
	"errors"
	"fmt"
	"io"
	"net/url"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strconv"
	"strings"
)

// Saved servers of other programs.
//
// The extension sends "sites" when the person presses "Import saved servers". The helper then looks for the lists that
// FileZilla, WinSCP and Cyberduck keep for the same person on the same computer, and hands back what a new connection
// needs: a name, the kind, the server, the port, the user name and the start folder.
//
// What this file never does:
//   - read a password, a key, or the name of a key file. The structures below have no field for them, so the text is
//     thrown away while the file is read;
//   - write or change any file;
//   - look anywhere but in the places named below, or send anything anywhere but to the extension that asked.

const (
	maxSitesPerSource = 500 // one message to Chrome holds 1 MB; this keeps the answer far below it
	maxSiteText       = 200 // characters kept of any name, host, user or folder
	maxBookmarkFiles  = 1000
)

// maxSiteFile is the largest list that is read (a variable only so a test can lower it).
var maxSiteFile int64 = 16 << 20

// importedSite is one saved server, ready to become a connection of the extension.
type importedSite struct {
	// Name is what the person called it in the other program (the host when it has no name).
	Name string `json:"name"`
	// Kind is "ftp", "ftps" (TLS from the first byte) or "sftp".
	Kind string `json:"kind"`
	// TLS is the same word the "open" request takes: "auto", "explicit", "implicit" or "none". Empty for SFTP.
	TLS  string `json:"tls,omitempty"`
	Host string `json:"host"`
	// Port is 0 when the program uses the usual port.
	Port int    `json:"port,omitempty"`
	User string `json:"user,omitempty"`
	Path string `json:"path,omitempty"`
	// Group is the folder the program keeps it in ("Clients / Acme"), for display only.
	Group string `json:"group,omitempty"`
}

// siteSource is what one program had.
type siteSource struct {
	ID    string `json:"id"`
	Name  string `json:"name"`
	Found bool   `json:"found"`
	// Sites are the servers the extension can open (FTP, FTPS, SFTP).
	Sites []importedSite `json:"sites"`
	// Skipped counts the entries that are another kind (WebDAV, cloud storage, SCP...) or have no server.
	Skipped   int  `json:"skipped"`
	Truncated bool `json:"truncated,omitempty"`
	// Error is a plain sentence when the list exists but could not be read.
	Error string `json:"error,omitempty"`
}

func newSource(id, name string) siteSource {
	return siteSource{ID: id, Name: name, Sites: []importedSite{}}
}

func (s *siteSource) add(site importedSite) {
	if len(s.Sites) >= maxSitesPerSource {
		s.Truncated = true
		return
	}
	if site.Name == "" {
		site.Name = site.Host
	}
	s.Sites = append(s.Sites, site)
}

func clip(text string) string {
	text = strings.TrimSpace(text)
	if r := []rune(text); len(r) > maxSiteText {
		return string(r[:maxSiteText])
	}
	return text
}

func portOf(text string) int {
	n, err := strconv.Atoi(strings.TrimSpace(text))
	if err != nil || n < 1 || n > 65535 {
		return 0
	}
	return n
}

func readSiteFile(path string) ([]byte, error) {
	info, err := os.Stat(path)
	if err != nil {
		return nil, err
	}
	if !info.Mode().IsRegular() {
		return nil, errors.New("it is not a file")
	}
	if info.Size() > maxSiteFile {
		return nil, errors.New("the file is too large")
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	return bytes.TrimPrefix(data, []byte{0xEF, 0xBB, 0xBF}), nil
}

// siteLocations says where the programs keep their lists. It is built from the environment each time it is asked, so a
// test can point it at a folder of sample files.
type siteLocations struct {
	appData string // Windows: %APPDATA%
	home    string
	config  string // Linux and macOS: $XDG_CONFIG_HOME, or ~/.config
}

func currentSiteLocations() siteLocations {
	home, _ := os.UserHomeDir()
	config := os.Getenv("XDG_CONFIG_HOME")
	if config == "" && home != "" {
		config = filepath.Join(home, ".config")
	}
	return siteLocations{appData: os.Getenv("APPDATA"), home: home, config: config}
}

func (l siteLocations) filezillaFiles() []string {
	var out []string
	if runtime.GOOS == "windows" {
		if l.appData != "" {
			out = append(out, filepath.Join(l.appData, "FileZilla", "sitemanager.xml"))
		}
		return out
	}
	if l.config != "" {
		out = append(out, filepath.Join(l.config, "filezilla", "sitemanager.xml"))
	}
	if l.home != "" {
		out = append(out, filepath.Join(l.home, ".filezilla", "sitemanager.xml"))
		if runtime.GOOS == "darwin" {
			out = append(out, filepath.Join(l.home, "Library", "Application Support", "FileZilla", "sitemanager.xml"))
		}
	}
	return out
}

func (l siteLocations) winscpFile() string {
	if l.appData == "" {
		return ""
	}
	return filepath.Join(l.appData, "WinSCP.ini")
}

func (l siteLocations) cyberduckDir() string {
	switch runtime.GOOS {
	case "windows":
		if l.appData != "" {
			return filepath.Join(l.appData, "Cyberduck", "Bookmarks")
		}
	case "darwin":
		if l.home != "" {
			return filepath.Join(l.home, "Library", "Application Support", "Cyberduck", "Bookmarks")
		}
	}
	return ""
}

// discoverSites asks every program that exists on this kind of computer.
func discoverSites(l siteLocations) []siteSource {
	sources := []siteSource{readFileZilla(l)}
	if runtime.GOOS == "windows" {
		sources = append(sources, readWinSCP(l, winscpRegistryRoot))
	}
	if runtime.GOOS == "windows" || runtime.GOOS == "darwin" {
		sources = append(sources, readCyberduck(l))
	}
	return sources
}

// ---- FileZilla: sitemanager.xml -------------------------------------------------------------------------------------

type fzDocument struct {
	Servers fzFolder `xml:"Servers"`
}

// fzFolder is a folder of the Site Manager. Its name is the text before the first element inside it.
type fzFolder struct {
	Name    string     `xml:",chardata"`
	Servers []fzServer `xml:"Server"`
	Folders []fzFolder `xml:"Folder"`
}

// fzServer holds only what an import needs. FileZilla also writes <Pass> (the password) and <Keyfile>: they have no
// field here, so they are never kept.
type fzServer struct {
	Host      string `xml:"Host"`
	Port      string `xml:"Port"`
	Protocol  string `xml:"Protocol"`
	User      string `xml:"User"`
	Logontype string `xml:"Logontype"`
	Name      string `xml:"Name"`
	RemoteDir string `xml:"RemoteDir"`
}

func readFileZilla(l siteLocations) siteSource {
	out := newSource("filezilla", "FileZilla")
	for _, path := range l.filezillaFiles() {
		if _, err := os.Stat(path); err != nil {
			continue
		}
		out.Found = true
		data, err := readSiteFile(path)
		if err != nil {
			out.Error = "The list of FileZilla could not be read: " + shortReason(err)
			return out
		}
		if err := parseFileZilla(data, &out); err != nil {
			out.Error = "The list of FileZilla could not be read: " + shortReason(err)
		}
		return out
	}
	return out
}

func parseFileZilla(data []byte, out *siteSource) error {
	var doc fzDocument
	if err := xml.Unmarshal(data, &doc); err != nil {
		return err
	}
	walkFileZilla(doc.Servers, nil, out)
	return nil
}

func walkFileZilla(folder fzFolder, group []string, out *siteSource) {
	where := strings.Join(group, " / ")
	for _, s := range folder.Servers {
		addFileZillaSite(s, where, out)
	}
	for _, f := range folder.Folders {
		next := group
		if name := clip(f.Name); name != "" {
			next = append(append([]string{}, group...), name)
		}
		walkFileZilla(f, next, out)
	}
}

func addFileZillaSite(s fzServer, group string, out *siteSource) {
	site := importedSite{
		Name:  clip(s.Name),
		Host:  clip(s.Host),
		Port:  portOf(s.Port),
		User:  clip(s.User),
		Path:  clip(fzRemotePath(s.RemoteDir)),
		Group: group,
	}
	if site.Host == "" {
		out.Skipped++
		return
	}
	// The numbers are FileZilla's own: 0 FTP (explicit TLS when the server can), 1 SFTP, 3 FTPS (TLS from the first byte),
	// 4 FTP with explicit TLS required, 6 plain FTP only. Others are web, cloud and storage services.
	switch strings.TrimSpace(s.Protocol) {
	case "", "0":
		site.Kind, site.TLS = "ftp", "auto"
	case "1":
		site.Kind = "sftp"
	case "3":
		site.Kind, site.TLS = "ftps", "implicit"
	case "4":
		site.Kind, site.TLS = "ftp", "explicit"
	case "6":
		site.Kind, site.TLS = "ftp", "none"
	default:
		out.Skipped++
		return
	}
	if site.User == "" && site.Kind != "sftp" && strings.TrimSpace(s.Logontype) == "0" {
		site.User = "anonymous"
	}
	out.add(site)
}

// fzRemotePath reads the way FileZilla writes a folder: the kind of server, the length of a prefix (0 for none), then
// every part of the path as its length and its text. "1 0 4 home 3 www" is /home/www. Only the kinds of server that use
// "/" are read; for any other the person starts in the home folder.
func fzRemotePath(text string) string {
	kind, rest, ok := fzNumber(text)
	if !ok {
		return ""
	}
	switch kind {
	case 0, 1, 9: // default, Unix, Cygwin
	default:
		return ""
	}
	prefix, rest, ok := fzNumber(rest)
	if !ok {
		return ""
	}
	if prefix > 0 {
		runes := []rune(rest)
		if prefix > len(runes) {
			return ""
		}
		rest = strings.TrimPrefix(string(runes[prefix:]), " ")
	}
	var parts []string
	for strings.TrimSpace(rest) != "" {
		n, after, ok := fzNumber(rest)
		runes := []rune(after)
		if !ok || n > len(runes) {
			return ""
		}
		parts = append(parts, string(runes[:n]))
		rest = strings.TrimPrefix(string(runes[n:]), " ")
	}
	return "/" + strings.Join(parts, "/")
}

// fzNumber takes the number at the start of the text and returns what follows it (after one space).
func fzNumber(text string) (int, string, bool) {
	text = strings.TrimLeft(text, " ")
	token, rest := text, ""
	if i := strings.IndexByte(text, ' '); i >= 0 {
		token, rest = text[:i], text[i+1:]
	}
	n, err := strconv.Atoi(token)
	if err != nil || n < 0 {
		return 0, "", false
	}
	return n, rest, true
}

// ---- WinSCP: WinSCP.ini and, on Windows, the registry ---------------------------------------------------------------

// winscpRegistryRoot is where WinSCP keeps its sessions unless the person chose an INI file (a variable only so a test
// can point it at a key of its own, and never read the real sessions of the person who runs the tests).
var winscpRegistryRoot = `Software\Martin Prikryl\WinSCP 2\Sessions`

// winscpFields are the only values read from a session. Password, key file and the rest are never asked for.
var winscpFields = []string{"HostName", "PortNumber", "UserName", "FSProtocol", "Ftps", "RemoteDirectory", "IsWorkspace"}

// winscpSession is one saved session: its name (as stored, so possibly with %20 and "/" for folders) and its values.
type winscpSession struct {
	Name   string
	Values map[string]string
}

func readWinSCP(l siteLocations, registryRoot string) siteSource {
	out := newSource("winscp", "WinSCP")
	var sessions []winscpSession
	var problems []string

	if path := l.winscpFile(); path != "" {
		if _, err := os.Stat(path); err == nil {
			out.Found = true
			data, err := readSiteFile(path)
			if err == nil {
				var parsed []winscpSession
				parsed, err = parseWinSCPIni(data)
				sessions = append(sessions, parsed...)
			}
			if err != nil {
				problems = append(problems, shortReason(err))
			}
		}
	}
	registrySessions, found, err := readWinSCPRegistry(registryRoot)
	if found {
		out.Found = true
	}
	if err != nil {
		problems = append(problems, shortReason(err))
	}
	sessions = append(sessions, registrySessions...)

	for _, s := range sessions {
		addWinSCPSite(s, &out)
	}
	if len(problems) > 0 && len(out.Sites) == 0 {
		out.Error = "The list of WinSCP could not be read: " + problems[0]
	}
	return out
}

// parseWinSCPIni reads the [Sessions\...] parts of WinSCP.ini and nothing else.
func parseWinSCPIni(data []byte) ([]winscpSession, error) {
	if len(data) >= 2 && (data[0] == 0xFF && data[1] == 0xFE || data[0] == 0xFE && data[1] == 0xFF) {
		return nil, errors.New("the file is saved as UTF-16, which is not read")
	}
	wanted := map[string]bool{}
	for _, f := range winscpFields {
		wanted[f] = true
	}
	var sessions []winscpSession
	var current *winscpSession
	for _, line := range strings.Split(string(data), "\n") {
		line = strings.TrimSpace(line)
		switch {
		case line == "" || strings.HasPrefix(line, ";") || strings.HasPrefix(line, "#"):
		case strings.HasPrefix(line, "[") && strings.HasSuffix(line, "]"):
			section := line[1 : len(line)-1]
			if name, ok := strings.CutPrefix(section, `Sessions\`); ok {
				sessions = append(sessions, winscpSession{Name: name, Values: map[string]string{}})
				current = &sessions[len(sessions)-1]
			} else {
				current = nil
			}
		case current != nil:
			key, value, ok := strings.Cut(line, "=")
			key = strings.TrimSpace(key)
			if ok && wanted[key] {
				current.Values[key] = value
			}
		}
	}
	return sessions, nil
}

// unescape undoes the %20 style escapes WinSCP uses in names and values; text that is not valid stays as it is.
func unescape(text string) string {
	if !strings.Contains(text, "%") {
		return text
	}
	if decoded, err := url.PathUnescape(text); err == nil {
		return decoded
	}
	return text
}

func addWinSCPSite(s winscpSession, out *siteSource) {
	if strings.TrimSpace(s.Values["IsWorkspace"]) == "1" {
		return // a group of sessions, not a server
	}
	host := clip(unescape(s.Values["HostName"]))
	if host == "" {
		return // the "Default Settings" entry, and sessions that were never filled in
	}
	name := unescape(s.Name)
	group := ""
	if i := strings.LastIndexByte(name, '/'); i >= 0 {
		name, group = name[i+1:], strings.ReplaceAll(name[:i], "/", " / ")
	}
	site := importedSite{
		Name:  clip(name),
		Host:  host,
		Port:  portOf(s.Values["PortNumber"]),
		User:  clip(unescape(s.Values["UserName"])),
		Path:  clip(unescape(s.Values["RemoteDirectory"])),
		Group: clip(group),
	}
	// FSProtocol is WinSCP's own number: 0 SCP, 1 and 2 SFTP, 5 FTP, 6 WebDAV, 7 Amazon S3. It is left out of the file
	// when it is the default, which is SFTP.
	protocol := 1
	if text := strings.TrimSpace(s.Values["FSProtocol"]); text != "" {
		n, err := strconv.Atoi(text)
		if err != nil {
			out.Skipped++
			return
		}
		protocol = n
	}
	switch protocol {
	case 1, 2:
		site.Kind = "sftp"
	case 5:
		// Ftps: 0 no encryption, 1 TLS from the first byte, 2 and 3 explicit TLS.
		switch strings.TrimSpace(s.Values["Ftps"]) {
		case "1":
			site.Kind, site.TLS = "ftps", "implicit"
		case "2", "3":
			site.Kind, site.TLS = "ftp", "explicit"
		default:
			site.Kind, site.TLS = "ftp", "auto"
		}
	default:
		out.Skipped++
		return
	}
	out.add(site)
}

// ---- Cyberduck: Bookmarks/*.duck ------------------------------------------------------------------------------------

func readCyberduck(l siteLocations) siteSource {
	out := newSource("cyberduck", "Cyberduck")
	dir := l.cyberduckDir()
	if dir == "" {
		return out
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		if !errors.Is(err, os.ErrNotExist) {
			out.Found = true
			out.Error = "The bookmarks of Cyberduck could not be read: " + shortReason(err)
		}
		return out
	}
	out.Found = true
	names := make([]string, 0, len(entries))
	for _, e := range entries {
		if !e.IsDir() && strings.EqualFold(filepath.Ext(e.Name()), ".duck") {
			names = append(names, e.Name())
		}
	}
	sort.Strings(names)
	if len(names) > maxBookmarkFiles {
		names = names[:maxBookmarkFiles]
		out.Truncated = true
	}
	for _, name := range names {
		data, err := readSiteFile(filepath.Join(dir, name))
		if err != nil {
			out.Skipped++
			continue
		}
		values, err := parsePlist(data, cyberduckFields)
		if err != nil {
			out.Skipped++
			continue
		}
		addCyberduckSite(values, &out)
	}
	return out
}

func addCyberduckSite(values map[string]string, out *siteSource) {
	site := importedSite{
		Name: clip(values["Nickname"]),
		Host: clip(values["Hostname"]),
		Port: portOf(values["Port"]),
		User: clip(values["Username"]),
		Path: clip(firstOf(values["Path"], values["Default Path"])),
	}
	if site.Host == "" {
		out.Skipped++
		return
	}
	switch strings.TrimSpace(values["Protocol"]) {
	case "ftp":
		site.Kind, site.TLS = "ftp", "auto"
	case "ftp-tls":
		site.Kind, site.TLS = "ftp", "explicit"
	case "sftp":
		site.Kind = "sftp"
	default:
		out.Skipped++
		return
	}
	out.add(site)
}

func firstOf(texts ...string) string {
	for _, t := range texts {
		if strings.TrimSpace(t) != "" {
			return t
		}
	}
	return ""
}

// cyberduckFields are the only values kept from a bookmark. A key file, a password or a label is never kept.
var cyberduckFields = []string{"Protocol", "Hostname", "Port", "Username", "Path", "Default Path", "Nickname"}

// parsePlist reads the first level of an Apple property list in XML form: every <key> followed by a <string> or an
// <integer>. Only the keys named in `wanted` are kept; lists and nested dictionaries are skipped.
func parsePlist(data []byte, wanted []string) (map[string]string, error) {
	keep := map[string]bool{}
	for _, name := range wanted {
		keep[name] = true
	}
	decoder := xml.NewDecoder(bytes.NewReader(data))
	values := map[string]string{}
	depth := 0
	key := ""
	for {
		token, err := decoder.Token()
		if err != nil {
			if errors.Is(err, io.EOF) {
				break
			}
			return nil, err
		}
		switch t := token.(type) {
		case xml.StartElement:
			depth++
			switch {
			case depth == 3 && t.Name.Local == "key":
				var text string
				if err := decoder.DecodeElement(&text, &t); err != nil {
					return nil, err
				}
				depth--
				key = text
			case depth == 3 && key != "":
				switch t.Name.Local {
				case "string", "integer", "real":
					var text string
					if err := decoder.DecodeElement(&text, &t); err != nil {
						return nil, err
					}
					if keep[key] {
						values[key] = text
					}
				default:
					if err := decoder.Skip(); err != nil {
						return nil, err
					}
				}
				depth--
				key = ""
			}
		case xml.EndElement:
			depth--
		}
	}
	if len(values) == 0 {
		return nil, errors.New("it holds no bookmark")
	}
	return values, nil
}

// shortReason turns an error from the file system or the parser into a short sentence with no folder name in it.
func shortReason(err error) string {
	switch {
	case errors.Is(err, os.ErrPermission):
		return "the file is not allowed to be read."
	case errors.Is(err, os.ErrNotExist):
		return "the file is not there any more."
	}
	var syntax *xml.SyntaxError
	if errors.As(err, &syntax) {
		return "the file is not in the form this version knows."
	}
	return strings.TrimSuffix(fmt.Sprint(unwrapPath(err)), ".") + "."
}

// unwrapPath drops the path that file system errors carry (it holds the person's user name).
func unwrapPath(err error) error {
	var pe *os.PathError
	if errors.As(err, &pe) {
		return pe.Err
	}
	return err
}

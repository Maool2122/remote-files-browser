package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"strings"
	"testing"
)

// Texts that must never come out of the helper. The sample files below hold them on purpose.
var neverSent = []string{"secret-password", "c2VjcmV0", "SECRETHASH", "id.ppk", "id.pem", "decoy.example.com", "Keyfile", "Password", "Pass"}

func assertNothingSecret(t *testing.T, value any) {
	t.Helper()
	data, err := json.Marshal(value)
	if err != nil {
		t.Fatal(err)
	}
	for _, word := range neverSent {
		if strings.Contains(string(data), word) {
			t.Errorf("the answer holds %q, which is a password or a key: %s", word, data)
		}
	}
}

const sampleFileZilla = `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>
<FileZilla3 version="3.66.5" platform="windows">
	<Servers>
		<Server>
			<Host>ftp.example.com</Host>
			<Port>21</Port>
			<Protocol>0</Protocol>
			<Type>0</Type>
			<User>alice</User>
			<Pass encoding="base64">c2VjcmV0LXBhc3N3b3Jk</Pass>
			<Logontype>1</Logontype>
			<EncodingType>Auto</EncodingType>
			<Name>Plain site</Name>
			<RemoteDir>1 0 4 home 5 alice</RemoteDir>
		</Server>
		<Folder expanded="1">Clients
			<Server>
				<Host>sftp.example.com</Host>
				<Port>22</Port>
				<Protocol>1</Protocol>
				<User>bob</User>
				<Logontype>5</Logontype>
				<Keyfile>C:\keys\id.ppk</Keyfile>
				<Name>Work SFTP</Name>
				<RemoteDir>1 0 4 Café 3 ünï</RemoteDir>
			</Server>
			<Folder expanded="0">Acme
				<Server>
					<Host>ftps.example.com</Host>
					<Port>990</Port>
					<Protocol>3</Protocol>
					<User>carol</User>
					<Pass encoding="base64">c2VjcmV0LXBhc3N3b3Jk</Pass>
					<Name>Implicit TLS</Name>
				</Server>
				<Server>
					<Host>explicit.example.com</Host>
					<Protocol>4</Protocol>
					<User>dave</User>
					<Name>Explicit TLS</Name>
				</Server>
			</Folder>
		</Folder>
		<Server>
			<Host>files.example.org</Host>
			<Protocol>6</Protocol>
			<Logontype>0</Logontype>
		</Server>
		<Server>
			<Host>bucket.example.org</Host>
			<Protocol>7</Protocol>
			<Name>A storage service</Name>
		</Server>
		<Server>
			<Host></Host>
			<Name>Nothing filled in</Name>
		</Server>
	</Servers>
</FileZilla3>
`

func TestFileZillaList(t *testing.T) {
	out := newSource("filezilla", "FileZilla")
	if err := parseFileZilla([]byte(sampleFileZilla), &out); err != nil {
		t.Fatal(err)
	}
	// The servers of a level come first, then its folders (so the ones with no folder are at the top).
	want := []importedSite{
		{Name: "Plain site", Kind: "ftp", TLS: "auto", Host: "ftp.example.com", Port: 21, User: "alice", Path: "/home/alice"},
		{Name: "files.example.org", Kind: "ftp", TLS: "none", Host: "files.example.org", User: "anonymous"},
		{Name: "Work SFTP", Kind: "sftp", Host: "sftp.example.com", Port: 22, User: "bob", Path: "/Café/ünï", Group: "Clients"},
		{Name: "Implicit TLS", Kind: "ftps", TLS: "implicit", Host: "ftps.example.com", Port: 990, User: "carol", Group: "Clients / Acme"},
		{Name: "Explicit TLS", Kind: "ftp", TLS: "explicit", Host: "explicit.example.com", User: "dave", Group: "Clients / Acme"},
	}
	if !reflect.DeepEqual(out.Sites, want) {
		t.Fatalf("sites =\n%+v\nwant\n%+v", out.Sites, want)
	}
	if out.Skipped != 2 {
		t.Errorf("skipped = %d, want 2 (a storage service and an entry with no server)", out.Skipped)
	}
	assertNothingSecret(t, out)
}

func TestFileZillaPath(t *testing.T) {
	cases := map[string]string{
		"":                    "",
		"1 0":                 "/",
		"1 0 4 home 3 www":    "/home/www",
		"0 0 4 home 3 www":    "/home/www",
		"1 0 4 Café 3 ünï":    "/Café/ünï",
		"1 0 7 my site 1 x":   "/my site/x",
		"3 0 4 home":          "", // DOS paths do not use "/": the person starts in the home folder
		"1 0 9 short":         "", // the length is longer than the text
		"1 0 x":               "",
		"nonsense":            "",
		"1 2 AB 4 home":       "/home", // a prefix of two characters is skipped
		"1 0 4 home 0  3 www": "/home//www",
	}
	for text, want := range cases {
		if got := fzRemotePath(text); got != want {
			t.Errorf("fzRemotePath(%q) = %q, want %q", text, got, want)
		}
	}
}

func TestFileZillaBrokenFile(t *testing.T) {
	out := newSource("filezilla", "FileZilla")
	if err := parseFileZilla([]byte("<FileZilla3><Servers><Server>"), &out); err == nil {
		t.Fatal("a file that stops in the middle must be an error, not an empty list")
	}
	if len(out.Sites) != 0 {
		t.Fatalf("sites = %v", out.Sites)
	}
}

const sampleWinSCP = "\xEF\xBB\xBF" + `[Configuration\Interface]
Theme=Dark

[Sessions\Default%20Settings]
FSProtocol=1
PortNumber=22

[Sessions\Office%20server]
HostName=sftp.example.com
UserName=carol
Password=SECRETHASH
PublicKeyFile=C%3A%5Ckeys%5Cid.ppk
RemoteDirectory=/var/www

[Sessions\Clients/Acme%20FTP]
HostName=ftp.acme.test
UserName=dave
FSProtocol=5
Ftps=3
PortNumber=2121
Password=SECRETHASH

[Sessions\Plain%20FTP]
HostName=ftp.plain.test
FSProtocol=5

[Sessions\Implicit]
HostName=ftps.example.com
FSProtocol=5
Ftps=1

[Sessions\Dav]
HostName=dav.example.com
FSProtocol=6

[Sessions\Scp]
HostName=scp.example.com
FSProtocol=0

[Sessions\Group%20of%20sessions]
IsWorkspace=1
`

// noRegistry names a key that does not exist, so a test never reads the real WinSCP sessions of the person who runs it.
const noRegistry = `Software\RocuoHelperTest\NoSuchKey`

func TestWinSCPList(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "WinSCP.ini"), []byte(sampleWinSCP), 0o600); err != nil {
		t.Fatal(err)
	}
	out := readWinSCP(siteLocations{appData: dir}, noRegistry)
	if !out.Found || out.Error != "" {
		t.Fatalf("found = %v, error = %q", out.Found, out.Error)
	}
	want := []importedSite{
		{Name: "Office server", Kind: "sftp", Host: "sftp.example.com", User: "carol", Path: "/var/www"},
		{Name: "Acme FTP", Kind: "ftp", TLS: "explicit", Host: "ftp.acme.test", Port: 2121, User: "dave", Group: "Clients"},
		{Name: "Plain FTP", Kind: "ftp", TLS: "auto", Host: "ftp.plain.test"},
		{Name: "Implicit", Kind: "ftps", TLS: "implicit", Host: "ftps.example.com"},
	}
	if !reflect.DeepEqual(out.Sites, want) {
		t.Fatalf("sites =\n%+v\nwant\n%+v", out.Sites, want)
	}
	if out.Skipped != 2 {
		t.Errorf("skipped = %d, want 2 (WebDAV and SCP)", out.Skipped)
	}
	assertNothingSecret(t, out)
}

func TestWinSCPNotThere(t *testing.T) {
	out := readWinSCP(siteLocations{appData: t.TempDir()}, noRegistry)
	if out.Found || len(out.Sites) != 0 || out.Error != "" {
		t.Fatalf("a computer without WinSCP gives %+v", out)
	}
	if out.Sites == nil {
		t.Fatal("the list of sites must be an empty list, not null, in the answer")
	}
}

func TestWinSCPUnreadableFile(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "WinSCP.ini"), []byte("\xFF\xFE[\x00S\x00"), 0o600); err != nil {
		t.Fatal(err)
	}
	out := readWinSCP(siteLocations{appData: dir}, noRegistry)
	if !out.Found || out.Error == "" || len(out.Sites) != 0 {
		t.Fatalf("a UTF-16 file must be reported, not guessed at: %+v", out)
	}
	if strings.Contains(out.Error, dir) {
		t.Fatalf("the sentence must not hold a folder name: %q", out.Error)
	}
}

const sampleDuck = `<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0">
<dict>
	<key>Protocol</key>
	<string>sftp</string>
	<key>Provider</key>
	<string>iterate GmbH</string>
	<key>Hostname</key>
	<string>sftp.example.com</string>
	<key>Port</key>
	<string>2222</string>
	<key>Username</key>
	<string>erin</string>
	<key>Path</key>
	<string>/srv/files</string>
	<key>Nickname</key>
	<string>Erin &amp; Co</string>
	<key>Labels</key>
	<array>
		<string>blue</string>
	</array>
	<key>Private Key File</key>
	<string>C:\keys\id.pem</string>
	<key>Readonly</key>
	<false/>
	<key>Extra</key>
	<dict>
		<key>Hostname</key>
		<string>decoy.example.com</string>
	</dict>
</dict>
</plist>
`

func TestCyberduckBookmark(t *testing.T) {
	values, err := parsePlist([]byte(sampleDuck), cyberduckFields)
	if err != nil {
		t.Fatal(err)
	}
	if values["Hostname"] != "sftp.example.com" {
		t.Errorf("a value inside a nested dictionary must not replace the real one: %v", values)
	}
	if _, kept := values["Private Key File"]; kept {
		t.Errorf("the name of a key file must not be kept: %v", values)
	}
	out := newSource("cyberduck", "Cyberduck")
	addCyberduckSite(values, &out)
	want := []importedSite{{Name: "Erin & Co", Kind: "sftp", Host: "sftp.example.com", Port: 2222, User: "erin", Path: "/srv/files"}}
	if !reflect.DeepEqual(out.Sites, want) {
		t.Fatalf("sites = %+v, want %+v", out.Sites, want)
	}
	assertNothingSecret(t, out)
}

func TestCyberduckKinds(t *testing.T) {
	cases := []struct {
		protocol, kind, tls string
	}{{"ftp", "ftp", "auto"}, {"ftp-tls", "ftp", "explicit"}, {"sftp", "sftp", ""}, {"dav", "", ""}, {"s3", "", ""}, {"", "", ""}}
	for _, c := range cases {
		out := newSource("cyberduck", "Cyberduck")
		addCyberduckSite(map[string]string{"Protocol": c.protocol, "Hostname": "h.example.com"}, &out)
		if c.kind == "" {
			if len(out.Sites) != 0 || out.Skipped != 1 {
				t.Errorf("%q must be skipped: %+v", c.protocol, out)
			}
			continue
		}
		if len(out.Sites) != 1 || out.Sites[0].Kind != c.kind || out.Sites[0].TLS != c.tls {
			t.Errorf("%q = %+v, want %s/%s", c.protocol, out.Sites, c.kind, c.tls)
		}
	}
}

func TestCyberduckFolder(t *testing.T) {
	loc := siteLocations{appData: t.TempDir(), home: t.TempDir()}
	dir := loc.cyberduckDir()
	if dir == "" {
		t.Skip("Cyberduck is not read on " + runtime.GOOS)
	}
	if err := os.MkdirAll(dir, 0o700); err != nil {
		t.Fatal(err)
	}
	write := func(name, text string) {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(text), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	write("a.duck", sampleDuck)
	write("broken.duck", "<plist><dict><key>Hostname")
	write("notes.txt", sampleDuck)
	out := readCyberduck(loc)
	if !out.Found || len(out.Sites) != 1 || out.Skipped != 1 {
		t.Fatalf("one good bookmark, one broken, one other file: %+v", out)
	}
}

func TestFileTooLargeIsNotRead(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "WinSCP.ini")
	if err := os.WriteFile(path, []byte(sampleWinSCP), 0o600); err != nil {
		t.Fatal(err)
	}
	old := maxSiteFile
	maxSiteFile = 100
	defer func() { maxSiteFile = old }()
	out := readWinSCP(siteLocations{appData: dir}, noRegistry)
	if out.Error == "" || len(out.Sites) != 0 {
		t.Fatalf("a list larger than the limit must not be read: %+v", out)
	}
}

func TestTooManySitesAreCut(t *testing.T) {
	var b strings.Builder
	b.WriteString("<FileZilla3><Servers>")
	for i := 0; i < maxSitesPerSource+25; i++ {
		b.WriteString("<Server><Host>h.example.com</Host><Protocol>1</Protocol><User>u</User></Server>")
	}
	b.WriteString("</Servers></FileZilla3>")
	out := newSource("filezilla", "FileZilla")
	if err := parseFileZilla([]byte(b.String()), &out); err != nil {
		t.Fatal(err)
	}
	if len(out.Sites) != maxSitesPerSource || !out.Truncated {
		t.Fatalf("%d sites, truncated = %v", len(out.Sites), out.Truncated)
	}
}

func TestLongTextIsCut(t *testing.T) {
	out := newSource("filezilla", "FileZilla")
	long := strings.Repeat("é", maxSiteText+50)
	addFileZillaSite(fzServer{Host: long, Protocol: "1", User: "u"}, "", &out)
	if got := []rune(out.Sites[0].Host); len(got) != maxSiteText {
		t.Fatalf("host has %d characters", len(got))
	}
}

// The whole way: the extension asks "sites", the helper answers from the places the programs keep their lists.
func TestSitesOperation(t *testing.T) {
	home := t.TempDir()
	t.Setenv("APPDATA", filepath.Join(home, "AppData"))
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home)
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(home, ".config"))
	oldRoot := winscpRegistryRoot
	winscpRegistryRoot = noRegistry
	t.Cleanup(func() { winscpRegistryRoot = oldRoot })

	h := newHarness(t)
	empty := h.mustOK(map[string]any{"op": "sites"})
	for _, s := range decodeSources(t, empty) {
		if s.Found || len(s.Sites) != 0 {
			t.Errorf("nothing is installed, but %s answered %+v", s.ID, s)
		}
	}

	files := currentSiteLocations().filezillaFiles()
	if len(files) == 0 {
		t.Fatal("this system has no place for the list of FileZilla")
	}
	if err := os.MkdirAll(filepath.Dir(files[0]), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(files[0], []byte(sampleFileZilla), 0o600); err != nil {
		t.Fatal(err)
	}
	reply := h.mustOK(map[string]any{"op": "sites"})
	var found *siteSource
	sources := decodeSources(t, reply)
	for i := range sources {
		if sources[i].ID == "filezilla" {
			found = &sources[i]
		}
	}
	if found == nil || !found.Found || len(found.Sites) != 5 || found.Skipped != 2 || found.Name != "FileZilla" {
		t.Fatalf("filezilla = %+v", found)
	}
	assertNothingSecret(t, reply)
}

func decodeSources(t *testing.T, r reply) []siteSource {
	t.Helper()
	data, err := json.Marshal(r["sources"])
	if err != nil {
		t.Fatal(err)
	}
	var sources []siteSource
	if err := json.Unmarshal(data, &sources); err != nil {
		t.Fatalf("sources = %s: %v", data, err)
	}
	if len(sources) == 0 {
		t.Fatal("the answer has no sources")
	}
	return sources
}

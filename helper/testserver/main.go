// Command testserver starts the test servers for the extension tests: FTP, FTP with explicit TLS, FTP with implicit
// TLS, FTP that allows only two connections, SFTP and WebDAV. It prints one line of JSON with their ports and
// fingerprints and runs until its standard input is closed. Everything stays on 127.0.0.1 and in memory.
//
//	go run ./testserver -bulk 2000
package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"

	ftpserver "github.com/fclairamb/ftpserverlib"
	"github.com/spf13/afero"

	"rocuo.com/remote-files-helper/testservers"
)

func seed(fs afero.Fs, bulk int) error {
	files := map[string]string{
		"/docs/readme.txt":         "Hello from the test server.\n",
		"/docs/sub/inner.txt":      "A file in a folder in a folder.\n",
		"/docs/with space/a b.txt": "Names with spaces work.\n",
	}
	for path, text := range files {
		if err := afero.WriteFile(fs, path, []byte(text), 0o644); err != nil {
			return err
		}
	}
	for i := 0; i < bulk; i++ {
		if err := afero.WriteFile(fs, fmt.Sprintf("/bulk/f%05d.txt", i), []byte(fmt.Sprintf("file %d\n", i)), 0o644); err != nil {
			return err
		}
	}
	return nil
}

// seedDAV puts the same files on the WebDAV server as on the FTP servers.
func seedDAV(s *testservers.WebDAV, bulk int) error {
	files := map[string]string{
		"/docs/readme.txt":         "Hello from the test server.\n",
		"/docs/sub/inner.txt":      "A file in a folder in a folder.\n",
		"/docs/with space/a b.txt": "Names with spaces work.\n",
	}
	for path, text := range files {
		if err := s.Seed(path, []byte(text)); err != nil {
			return err
		}
	}
	for i := 0; i < bulk; i++ {
		if err := s.Seed(fmt.Sprintf("/bulk/f%05d.txt", i), []byte(fmt.Sprintf("file %d\n", i))); err != nil {
			return err
		}
	}
	return nil
}

func main() {
	bulk := flag.Int("bulk", 0, "number of small files to put in /bulk of the FTP servers")
	flag.Parse()

	fail := func(err error) {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	plain, err := testservers.StartFTP(testservers.FTPOptions{})
	if err != nil {
		fail(err)
	}
	explicit, err := testservers.StartFTP(testservers.FTPOptions{TLS: true})
	if err != nil {
		fail(err)
	}
	implicit, err := testservers.StartFTP(testservers.FTPOptions{TLS: true, Mode: ftpserver.ImplicitEncryption})
	if err != nil {
		fail(err)
	}
	limited, err := testservers.StartFTP(testservers.FTPOptions{MaxConns: 2})
	if err != nil {
		fail(err)
	}
	secure, err := testservers.StartSFTP()
	if err != nil {
		fail(err)
	}
	dav, err := testservers.StartWebDAV()
	if err != nil {
		fail(err)
	}
	for _, s := range []*testservers.FTP{plain, explicit, implicit, limited} {
		if err := seed(s.FS, *bulk); err != nil {
			fail(err)
		}
	}
	if err := secure.Seed("/docs/readme.txt", []byte("Hello from the test server.\n")); err != nil {
		fail(err)
	}
	if err := seedDAV(dav, *bulk); err != nil {
		fail(err)
	}

	info := map[string]any{
		"user":     testservers.User,
		"password": testservers.Password,
		"ftp":      map[string]any{"host": plain.Host, "port": plain.Port},
		"ftpTls":   map[string]any{"host": explicit.Host, "port": explicit.Port, "fingerprint": explicit.CertFP},
		"ftps":     map[string]any{"host": implicit.Host, "port": implicit.Port, "fingerprint": implicit.CertFP},
		"ftpLimit": map[string]any{"host": limited.Host, "port": limited.Port},
		"sftp":     map[string]any{"host": secure.Host, "port": secure.Port, "fingerprint": secure.HostFP, "display": secure.HostDisplay},
		"webdav":   map[string]any{"host": dav.Host, "port": dav.Port},
	}
	if err := json.NewEncoder(os.Stdout).Encode(info); err != nil {
		fail(err)
	}
	_, _ = io.Copy(io.Discard, os.Stdin)
}

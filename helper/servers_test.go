package main

import (
	"testing"

	"github.com/spf13/afero"

	"rocuo.com/remote-files-helper/testservers"
)

// The servers for the tests live in the testservers package (the extension tests use the same ones). These small
// wrappers start them for a Go test and stop them when the test ends.

type ftpOptions = testservers.FTPOptions

type ftpTest struct {
	*testservers.FTP
	fs     afero.Fs
	host   string
	port   int
	certFP string
}

func startFTP(t *testing.T, opts ftpOptions) *ftpTest {
	t.Helper()
	s, err := testservers.StartFTP(opts)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(s.Close)
	return &ftpTest{FTP: s, fs: s.FS, host: s.Host, port: s.Port, certFP: s.CertFP}
}

func (s *ftpTest) peakConnections() int { return s.Peak() }

type sftpTest struct {
	*testservers.SFTP
	host         string
	port         int
	hostFP       string
	hostDisplay  string
	userKeyPEM   []byte
	protectedPEM []byte
}

func startSFTP(t *testing.T) *sftpTest {
	t.Helper()
	s, err := testservers.StartSFTP()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(s.Close)
	return &sftpTest{SFTP: s, host: s.Host, port: s.Port, hostFP: s.HostFP, hostDisplay: s.HostDisplay, userKeyPEM: s.UserKeyPEM, protectedPEM: s.ProtectedPEM}
}

// Package testservers holds small real servers (FTP, FTPS, SFTP, WebDAV) for testing the helper and the extension.
// They listen on 127.0.0.1 only, keep their files in memory and use made-up passwords. They are never part of the helper.
package testservers

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/sha256"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/hex"
	"errors"
	"math/big"
	"net"
	"strconv"
	"sync"
	"time"

	ftpserver "github.com/fclairamb/ftpserverlib"
	"github.com/spf13/afero"
)

// User and password of every test server.
const (
	User     = "alice"
	Password = "secret"
)

func sha256Hex(data []byte) string {
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])
}

// SelfSignedCert makes a certificate for 127.0.0.1 and returns it with its SHA-256 fingerprint (hex, lower case).
func SelfSignedCert() (tls.Certificate, string, error) {
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		return tls.Certificate{}, "", err
	}
	template := &x509.Certificate{
		SerialNumber: big.NewInt(time.Now().UnixNano()),
		Subject:      pkix.Name{CommonName: "127.0.0.1"},
		NotBefore:    time.Now().Add(-time.Hour),
		NotAfter:     time.Now().Add(24 * time.Hour),
		KeyUsage:     x509.KeyUsageDigitalSignature,
		ExtKeyUsage:  []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
		IPAddresses:  []net.IP{net.ParseIP("127.0.0.1")},
	}
	der, err := x509.CreateCertificate(rand.Reader, template, template, &key.PublicKey, key)
	if err != nil {
		return tls.Certificate{}, "", err
	}
	return tls.Certificate{Certificate: [][]byte{der}, PrivateKey: key}, sha256Hex(der), nil
}

// FTPOptions chooses what the FTP server offers.
type FTPOptions struct {
	TLS      bool                     // offer TLS (needed for explicit and implicit)
	Mode     ftpserver.TLSRequirement // ClearOrEncrypted (default), MandatoryEncryption or ImplicitEncryption
	MaxConns int                      // control connections allowed at once; 0 = no limit
}

// FTP is a running FTP server.
type FTP struct {
	FS     afero.Fs
	Host   string
	Port   int
	CertFP string // SHA-256 of the certificate when TLS is on

	srv   *ftpserver.FtpServer
	cert  tls.Certificate
	opts  FTPOptions
	inner net.Listener

	mu     sync.Mutex
	active int
	peak   int
}

// limitedListener answers "421 Too many connections" like real servers do when a client opens more control
// connections than the server allows, and counts the open ones.
type limitedListener struct {
	net.Listener
	s *FTP
}

type countedConn struct {
	net.Conn
	s    *FTP
	once sync.Once
}

func (c *countedConn) Close() error {
	c.once.Do(func() {
		c.s.mu.Lock()
		c.s.active--
		c.s.mu.Unlock()
	})
	return c.Conn.Close()
}

func (l *limitedListener) Accept() (net.Conn, error) {
	for {
		conn, err := l.Listener.Accept()
		if err != nil {
			return nil, err
		}
		l.s.mu.Lock()
		if l.s.opts.MaxConns > 0 && l.s.active >= l.s.opts.MaxConns {
			l.s.mu.Unlock()
			_, _ = conn.Write([]byte("421 Too many connections from your address.\r\n"))
			_ = conn.Close()
			continue
		}
		l.s.active++
		if l.s.active > l.s.peak {
			l.s.peak = l.s.active
		}
		l.s.mu.Unlock()
		return &countedConn{Conn: conn, s: l.s}, nil
	}
}

// StartFTP starts an FTP server on a free port of 127.0.0.1.
func StartFTP(opts FTPOptions) (*FTP, error) {
	s := &FTP{FS: afero.NewMemMapFs(), opts: opts}
	if opts.TLS {
		var err error
		if s.cert, s.CertFP, err = SelfSignedCert(); err != nil {
			return nil, err
		}
	}
	inner, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return nil, err
	}
	s.inner = inner
	s.srv = ftpserver.NewFtpServer(s)
	if err := s.srv.Listen(); err != nil {
		inner.Close()
		return nil, err
	}
	go func() { _ = s.srv.Serve() }()
	host, port, err := net.SplitHostPort(s.srv.Addr())
	if err != nil {
		return nil, err
	}
	s.Host = host
	s.Port, _ = strconv.Atoi(port)
	return s, nil
}

// Close stops the server.
func (s *FTP) Close() { _ = s.srv.Stop() }

// Peak is the largest number of control connections that were open at the same moment.
func (s *FTP) Peak() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.peak
}

func (s *FTP) GetSettings() (*ftpserver.Settings, error) {
	listener := s.inner
	if s.opts.Mode == ftpserver.ImplicitEncryption {
		config, err := s.GetTLSConfig()
		if err != nil {
			return nil, err
		}
		listener = tls.NewListener(s.inner, config)
	}
	return &ftpserver.Settings{
		Listener:    &limitedListener{Listener: listener, s: s},
		PublicHost:  "127.0.0.1",
		TLSRequired: s.opts.Mode,
		IdleTimeout: 120,
	}, nil
}

func (s *FTP) ClientConnected(ftpserver.ClientContext) (string, error) {
	return "Rocuo test server", nil
}

func (s *FTP) ClientDisconnected(ftpserver.ClientContext) {}

func (s *FTP) AuthUser(_ ftpserver.ClientContext, user, pass string) (ftpserver.ClientDriver, error) {
	if (user == User && pass == Password) || user == "anonymous" {
		return s.FS, nil
	}
	return nil, errors.New("bad credentials")
}

func (s *FTP) GetTLSConfig() (*tls.Config, error) {
	if !s.opts.TLS {
		return nil, errors.New("TLS is off")
	}
	return &tls.Config{Certificates: []tls.Certificate{s.cert}, MinVersion: tls.VersionTLS12}, nil
}

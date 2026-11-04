package main

import (
	"crypto/sha256"
	"crypto/tls"
	"crypto/x509"
	"encoding/hex"
	"errors"
	"net"
	"strings"
	"sync"

	"golang.org/x/crypto/ssh"
)

// The extension keeps the list of servers the person has trusted. A server proves who it is with a TLS certificate
// (FTPS) or an SSH host key (SFTP). The helper never decides alone: when it meets a certificate or a key it cannot check
// by itself and that is not on the trusted list, it stops and sends a "trust" error with the fingerprint.

func sha256Hex(data []byte) string {
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])
}

// trustHolder remembers the last identity problem of one connection, because the libraries hide it inside their own errors.
type trustHolder struct {
	mu  sync.Mutex
	err *trustError
}

func (h *trustHolder) set(err *trustError) {
	h.mu.Lock()
	h.err = err
	h.mu.Unlock()
}

func (h *trustHolder) get() *trustError {
	h.mu.Lock()
	defer h.mu.Unlock()
	return h.err
}

func trustedSet(list []string) map[string]bool {
	set := make(map[string]bool, len(list))
	for _, item := range list {
		set[strings.ToLower(strings.TrimSpace(item))] = true
	}
	return set
}

// tlsConfig makes the TLS settings of an FTPS connection. Certificates that the computer trusts pass; other
// certificates pass only when their SHA-256 fingerprint is on the trusted list.
func tlsConfig(host string, trusted []string, holder *trustHolder) *tls.Config {
	allowed := trustedSet(trusted)
	return &tls.Config{
		ServerName:         host,
		MinVersion:         tls.VersionTLS12,
		ClientSessionCache: tls.NewLRUClientSessionCache(16),
		// The checks happen in VerifyConnection so that they also run when a session is resumed.
		InsecureSkipVerify: true,
		VerifyConnection: func(state tls.ConnectionState) error {
			if len(state.PeerCertificates) == 0 {
				return errors.New("the server sent no certificate")
			}
			leaf := state.PeerCertificates[0]
			roots := x509.NewCertPool()
			for _, cert := range state.PeerCertificates[1:] {
				roots.AddCert(cert)
			}
			_, verifyErr := leaf.Verify(x509.VerifyOptions{DNSName: host, Intermediates: roots})
			if verifyErr == nil {
				return nil
			}
			fingerprint := sha256Hex(leaf.Raw)
			if allowed[fingerprint] {
				return nil
			}
			problem := &trustError{Fingerprint: fingerprint, Kind: "tls", Reason: describeCertificateError(leaf, verifyErr)}
			holder.set(problem)
			return problem
		},
	}
}

func describeCertificateError(leaf *x509.Certificate, err error) string {
	var unknown x509.UnknownAuthorityError
	var invalid x509.CertificateInvalidError
	var hostname x509.HostnameError
	switch {
	case errors.As(err, &hostname):
		return "the certificate is made for another name"
	case errors.As(err, &invalid) && invalid.Reason == x509.Expired:
		return "the certificate has expired or is not valid yet"
	case errors.As(err, &unknown):
		if leaf.Subject.String() == leaf.Issuer.String() {
			return "self-signed"
		}
		return "issued by an authority that is not known"
	}
	return "the certificate could not be checked"
}

// hostKeyCallback checks an SSH host key against the trusted list.
func hostKeyCallback(trusted []string, holder *trustHolder) ssh.HostKeyCallback {
	allowed := trustedSet(trusted)
	return func(_ string, _ net.Addr, key ssh.PublicKey) error {
		fingerprint := sha256Hex(key.Marshal())
		if allowed[fingerprint] {
			return nil
		}
		problem := &trustError{Fingerprint: fingerprint, Kind: key.Type(), Display: ssh.FingerprintSHA256(key)}
		holder.set(problem)
		return problem
	}
}

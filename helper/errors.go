package main

import (
	"context"
	"crypto/x509"
	"errors"
	"fmt"
	"io"
	"net"
	"net/textproto"
	"os"
	"strings"
	"syscall"

	"github.com/pkg/sftp"
	"golang.org/x/crypto/ssh"
)

// opError is a problem the extension can show to the person. `Code` tells the extension what to do about it:
//
//	auth       the login was refused: ask for the password again
//	trust      the identity of the server is new or changed: ask the person (Extra has fingerprint, fpKind, reason)
//	cancelled  the request was cancelled
//	session    the connection to the server was lost: open it again
//	generic    anything else: show the message
type opError struct {
	Code    string
	Message string
	Extra   map[string]any
}

func (e *opError) Error() string { return e.Message }

func newError(code, format string, args ...any) *opError {
	return &opError{Code: code, Message: fmt.Sprintf(format, args...)}
}

// trustError is raised while checking the identity of a server (an SSH host key or a TLS certificate).
type trustError struct {
	Fingerprint string // SHA-256 as lower case hex: this is what the extension stores and sends back
	Kind        string // "tls" or the SSH key type, for example "ssh-ed25519"
	Reason      string // for a certificate: why the computer does not trust it
	Display     string // how the person is used to read it (for SSH: "SHA256:..." as in OpenSSH)
}

func (e *trustError) Error() string {
	return "The identity of the server is not trusted yet (" + e.Kind + " " + e.Fingerprint + ")."
}

// wire turns any error into the object that goes to the extension.
func wire(err error) map[string]any {
	out := map[string]any{"code": "generic", "message": err.Error()}
	var op *opError
	var trust *trustError
	switch {
	case errors.As(err, &op):
		out["code"] = op.Code
		out["message"] = op.Message
		for k, v := range op.Extra {
			out[k] = v
		}
	case errors.As(err, &trust):
		out["code"] = "trust"
		out["message"] = trust.Error()
		out["fingerprint"] = trust.Fingerprint
		out["fpKind"] = trust.Kind
		if trust.Reason != "" {
			out["reason"] = trust.Reason
		}
		if trust.Display != "" {
			out["display"] = trust.Display
		}
	case errors.Is(err, context.Canceled):
		out["code"] = "cancelled"
		out["message"] = "Cancelled."
	default:
		code, message := classify(err)
		out["code"] = code
		out["message"] = message
	}
	return out
}

// classify gives a plain sentence for the errors of the network, FTP and SFTP libraries.
func classify(err error) (string, string) {
	// What the server itself answered comes first: its words may contain anything, also "connection closed".
	var ftpErr *textproto.Error
	if errors.As(err, &ftpErr) {
		switch ftpErr.Code {
		case 421:
			return "session", "The server closed the connection (" + cleanFTPText(ftpErr.Msg) + ")."
		case 530, 532:
			return "auth", "The server did not accept the user name or password."
		case 550:
			return "generic", "The server says no (550): " + cleanFTPText(ftpErr.Msg)
		case 553:
			return "generic", "The server does not accept this name: " + cleanFTPText(ftpErr.Msg)
		case 552, 452:
			return "generic", "The server has no more room: " + cleanFTPText(ftpErr.Msg)
		}
		return "generic", fmt.Sprintf("The server answered %d: %s", ftpErr.Code, cleanFTPText(ftpErr.Msg))
	}
	var status *sftp.StatusError
	if errors.As(err, &status) {
		switch status.Code {
		case 2: // no such file
			return "generic", "The file or folder was not found."
		case 3: // permission denied
			return "generic", "The server does not allow this."
		case 4:
			return "generic", "The server could not do this: " + status.Error()
		}
	}
	if errors.Is(err, os.ErrNotExist) {
		return "generic", "The file or folder was not found."
	}
	if errors.Is(err, os.ErrPermission) {
		return "generic", "The server does not allow this."
	}

	// Then the network.
	var dns *net.DNSError
	if errors.As(err, &dns) {
		return "generic", "The server name could not be found. Check how it is spelled and that you are online."
	}
	if errors.Is(err, syscall.ECONNREFUSED) {
		return "generic", "The server refused the connection. Check the address and the port."
	}
	var netErr net.Error
	if errors.As(err, &netErr) && netErr.Timeout() {
		return "generic", "The server did not answer in time."
	}
	text := err.Error()
	if errors.Is(err, io.EOF) || errors.Is(err, io.ErrUnexpectedEOF) || errors.Is(err, net.ErrClosed) ||
		errors.Is(err, syscall.ECONNRESET) || errors.Is(err, syscall.EPIPE) || errors.Is(err, os.ErrClosed) ||
		strings.Contains(text, "connection lost") || strings.Contains(text, "use of closed network connection") ||
		strings.Contains(text, "forcibly closed") || strings.Contains(text, "broken pipe") {
		return "session", "The connection to the server was lost (" + shorten(text) + ")."
	}

	var unknownAuthority x509.UnknownAuthorityError
	if errors.As(err, &unknownAuthority) {
		return "generic", "The certificate of the server is not trusted."
	}
	var sshErr *ssh.ExitError
	if errors.As(err, &sshErr) {
		return "generic", sshErr.Error()
	}
	if strings.Contains(text, "unable to authenticate") || strings.Contains(text, "no supported methods remain") {
		return "auth", "The server did not accept the user name or password."
	}
	return "generic", text
}

// shorten keeps the technical detail of an error short enough for one line of the screen.
func shorten(text string) string {
	text = strings.Join(strings.Fields(text), " ")
	if len(text) > 160 {
		text = text[:160] + "..."
	}
	return text
}

func cleanFTPText(text string) string {
	text = strings.TrimSpace(strings.ReplaceAll(text, "\n", " "))
	if text == "" {
		return "no details"
	}
	return text
}

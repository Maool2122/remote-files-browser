package testservers

import (
	"crypto/ed25519"
	"crypto/rand"
	"encoding/pem"
	"errors"
	"net"
	"strconv"
	"strings"

	"github.com/pkg/sftp"
	"golang.org/x/crypto/ssh"
)

// SFTP is a running SFTP server. User "alice" may log in with the password "secret", with UserKeyPEM, or with
// ProtectedPEM (the same key, protected by the passphrase "phrase").
type SFTP struct {
	Host         string
	Port         int
	HostFP       string // SHA-256 of the host key as lower case hex
	HostDisplay  string // the same, written as OpenSSH writes it
	UserKeyPEM   []byte
	ProtectedPEM []byte

	listener   net.Listener
	authorized ssh.PublicKey
	hostKey    ssh.PublicKey
	handlers   sftp.Handlers
}

// Passphrase of SFTP.ProtectedPEM.
const Passphrase = "phrase"

// StartSFTP starts an SFTP server on a free port of 127.0.0.1. Its files live in memory.
func StartSFTP() (*SFTP, error) {
	s := &SFTP{handlers: sftp.InMemHandler()}

	_, hostPriv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		return nil, err
	}
	hostSigner, err := ssh.NewSignerFromKey(hostPriv)
	if err != nil {
		return nil, err
	}
	s.hostKey = hostSigner.PublicKey()
	s.HostFP = sha256Hex(hostSigner.PublicKey().Marshal())
	s.HostDisplay = ssh.FingerprintSHA256(hostSigner.PublicKey())

	_, userPriv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		return nil, err
	}
	userSigner, err := ssh.NewSignerFromKey(userPriv)
	if err != nil {
		return nil, err
	}
	s.authorized = userSigner.PublicKey()
	block, err := ssh.MarshalPrivateKey(userPriv, "")
	if err != nil {
		return nil, err
	}
	s.UserKeyPEM = pem.EncodeToMemory(block)
	block, err = ssh.MarshalPrivateKeyWithPassphrase(userPriv, "", []byte(Passphrase))
	if err != nil {
		return nil, err
	}
	s.ProtectedPEM = pem.EncodeToMemory(block)

	config := &ssh.ServerConfig{
		PasswordCallback: func(meta ssh.ConnMetadata, password []byte) (*ssh.Permissions, error) {
			if meta.User() == User && string(password) == Password {
				return nil, nil
			}
			return nil, errors.New("denied")
		},
		PublicKeyCallback: func(meta ssh.ConnMetadata, key ssh.PublicKey) (*ssh.Permissions, error) {
			if meta.User() == User && string(key.Marshal()) == string(s.authorized.Marshal()) {
				return nil, nil
			}
			return nil, errors.New("denied")
		},
	}
	config.AddHostKey(hostSigner)

	s.listener, err = net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return nil, err
	}
	host, port, _ := net.SplitHostPort(s.listener.Addr().String())
	s.Host = host
	s.Port, _ = strconv.Atoi(port)

	go func() {
		for {
			conn, err := s.listener.Accept()
			if err != nil {
				return
			}
			go s.serve(conn, config)
		}
	}()
	return s, nil
}

// Close stops the server.
func (s *SFTP) Close() { _ = s.listener.Close() }

// Seed puts a file on the server (its folder is made when it is missing).
func (s *SFTP) Seed(path string, data []byte) error {
	conn, err := ssh.Dial("tcp", net.JoinHostPort(s.Host, strconv.Itoa(s.Port)), &ssh.ClientConfig{
		User:            User,
		Auth:            []ssh.AuthMethod{ssh.Password(Password)},
		HostKeyCallback: ssh.FixedHostKey(s.hostKey),
	})
	if err != nil {
		return err
	}
	defer conn.Close()
	client, err := sftp.NewClient(conn)
	if err != nil {
		return err
	}
	defer client.Close()
	if dir := path[:strings.LastIndex(path, "/")]; dir != "" {
		if err := client.MkdirAll(dir); err != nil {
			return err
		}
	}
	f, err := client.Create(path)
	if err != nil {
		return err
	}
	if _, err := f.Write(data); err != nil {
		f.Close()
		return err
	}
	return f.Close()
}

func (s *SFTP) serve(conn net.Conn, config *ssh.ServerConfig) {
	defer conn.Close()
	serverConn, channels, requests, err := ssh.NewServerConn(conn, config)
	if err != nil {
		return
	}
	defer serverConn.Close()
	go ssh.DiscardRequests(requests)
	for newChannel := range channels {
		if newChannel.ChannelType() != "session" {
			_ = newChannel.Reject(ssh.UnknownChannelType, "only sessions")
			continue
		}
		channel, reqs, err := newChannel.Accept()
		if err != nil {
			return
		}
		go func() {
			for req := range reqs {
				ok := req.Type == "subsystem" && len(req.Payload) > 4 && strings.HasSuffix(string(req.Payload), "sftp")
				_ = req.Reply(ok, nil)
				if ok {
					go func() {
						server := sftp.NewRequestServer(channel, s.handlers)
						_ = server.Serve()
						_ = channel.Close()
					}()
				}
			}
		}()
	}
}

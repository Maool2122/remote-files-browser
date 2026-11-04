package main

import (
	"context"
	"crypto/x509"
	"errors"
	"io"
	"net"
	"os"
	"path"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/pkg/sftp"
	"golang.org/x/crypto/ssh"
)

type sftpBackend struct {
	ssh     *ssh.Client
	client  *sftp.Client
	homeDir string
	stop    chan struct{}
	once    sync.Once
}

func openSFTP(ctx context.Context, cfg connectConfig) (backend, error) {
	holder := &trustHolder{}
	methods, err := sshAuthMethods(cfg)
	if err != nil {
		return nil, err
	}
	user := cfg.User
	if user == "" {
		return nil, newError("auth", "Enter the user name for this server.")
	}
	config := &ssh.ClientConfig{
		User:            user,
		Auth:            methods,
		HostKeyCallback: hostKeyCallback(cfg.Trusted, holder),
		Timeout:         20 * time.Second,
		ClientVersion:   "SSH-2.0-RocuoRemoteFiles",
	}
	address := net.JoinHostPort(cfg.Host, strconv.Itoa(cfg.Port))
	dialer := net.Dialer{Timeout: 20 * time.Second, KeepAlive: 30 * time.Second}
	conn, err := dialer.DialContext(ctx, "tcp", address)
	if err != nil {
		return nil, err
	}
	stopCancel := context.AfterFunc(ctx, func() { conn.Close() })
	sshConn, chans, reqs, err := ssh.NewClientConn(conn, address, config)
	if err != nil {
		stopCancel()
		conn.Close()
		if ctx.Err() != nil {
			return nil, ctx.Err()
		}
		if problem := holder.get(); problem != nil {
			return nil, problem
		}
		return nil, err
	}
	client := ssh.NewClient(sshConn, chans, reqs)
	if !stopCancel() && ctx.Err() != nil {
		client.Close()
		return nil, ctx.Err()
	}

	sc, err := sftp.NewClient(client,
		sftp.UseConcurrentReads(true),
		sftp.UseConcurrentWrites(true),
		sftp.MaxConcurrentRequestsPerFile(32),
	)
	if err != nil {
		client.Close()
		return nil, newError("generic", "The server does not offer SFTP (%s).", err.Error())
	}
	b := &sftpBackend{ssh: client, client: sc, stop: make(chan struct{})}
	if dir, derr := sc.Getwd(); derr == nil && dir != "" {
		b.homeDir = dir
	} else if real, rerr := sc.RealPath("."); rerr == nil && real != "" {
		b.homeDir = real
	} else {
		b.homeDir = "/"
	}
	go b.keepAlive()
	return b, nil
}

// keepAlive stops servers and routers from dropping a quiet connection.
func (b *sftpBackend) keepAlive() {
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-ticker.C:
			if _, _, err := b.ssh.SendRequest("keepalive@openssh.com", true, nil); err != nil {
				return
			}
		case <-b.stop:
			return
		}
	}
}

func sshAuthMethods(cfg connectConfig) ([]ssh.AuthMethod, error) {
	var methods []ssh.AuthMethod
	if strings.TrimSpace(cfg.KeyPath) != "" {
		signer, err := loadPrivateKey(cfg.KeyPath, cfg.Password)
		if err != nil {
			return nil, err
		}
		methods = append(methods, ssh.PublicKeys(signer))
		return methods, nil
	}
	if cfg.Password == "" {
		return nil, newError("auth", "Enter the password for this server, or choose a key file.")
	}
	password := cfg.Password
	methods = append(methods,
		ssh.Password(password),
		ssh.KeyboardInteractive(func(_, _ string, questions []string, _ []bool) ([]string, error) {
			answers := make([]string, len(questions))
			for i := range answers {
				answers[i] = password
			}
			return answers, nil
		}),
	)
	return methods, nil
}

func expandPath(p string) string {
	p = strings.TrimSpace(p)
	if p == "~" || strings.HasPrefix(p, "~/") || strings.HasPrefix(p, "~\\") {
		if home, err := os.UserHomeDir(); err == nil {
			return filepath.Join(home, p[1:])
		}
	}
	return os.ExpandEnv(p)
}

func loadPrivateKey(keyPath, passphrase string) (ssh.Signer, error) {
	data, err := os.ReadFile(expandPath(keyPath))
	if err != nil {
		return nil, newError("generic", "The key file could not be read: %s", filepath.Base(keyPath))
	}
	signer, err := ssh.ParsePrivateKey(data)
	if err == nil {
		return signer, nil
	}
	var missing *ssh.PassphraseMissingError
	if errors.As(err, &missing) {
		if passphrase == "" {
			return nil, newError("auth", "This key is protected by a passphrase. Enter the passphrase as the password.")
		}
		signer, err = ssh.ParsePrivateKeyWithPassphrase(data, []byte(passphrase))
		if err == nil {
			return signer, nil
		}
		if errors.Is(err, x509.IncorrectPasswordError) || strings.Contains(err.Error(), "decrypt") {
			return nil, newError("auth", "The passphrase of the key is not right.")
		}
	}
	return nil, newError("generic", "This file is not a private key that can be used (%s). Use an OpenSSH or PEM key.", err.Error())
}

func (b *sftpBackend) home() string { return b.homeDir }

func (b *sftpBackend) secure() (bool, string) { return true, "Encrypted (SSH)" }

func (b *sftpBackend) close() {
	b.once.Do(func() {
		close(b.stop)
		_ = b.client.Close()
		_ = b.ssh.Close()
	})
}

func modeText(mode os.FileMode) string {
	text := mode.Perm().String()
	if len(text) == 10 {
		return text[1:]
	}
	return text
}

func (b *sftpBackend) list(ctx context.Context, folder string, emit func([]entry) error) error {
	infos, err := b.client.ReadDir(folder)
	if err != nil {
		return err
	}
	batch := make([]entry, 0, ftpListBatch)
	for _, info := range infos {
		if err := ctx.Err(); err != nil {
			return err
		}
		name := info.Name()
		if name == "." || name == ".." || name == "" {
			continue
		}
		item := entry{Name: name, Dir: info.IsDir(), Size: info.Size(), Mode: modeText(info.Mode())}
		if mod := info.ModTime(); !mod.IsZero() && mod.Unix() > 0 {
			item.Modified = mod.UnixMilli()
		}
		if info.Mode()&os.ModeSymlink != 0 {
			item.Link = true
			// Follow the link to see if it leads to a folder.
			if target, serr := b.client.Stat(joinPath(folder, name)); serr == nil {
				item.Dir = target.IsDir()
				item.Size = target.Size()
				if mod := target.ModTime(); !mod.IsZero() && mod.Unix() > 0 {
					item.Modified = mod.UnixMilli()
				}
			}
		}
		if item.Dir {
			item.Size = -1
		}
		batch = append(batch, item)
		if len(batch) >= ftpListBatch {
			if err := emit(batch); err != nil {
				return err
			}
			batch = make([]entry, 0, ftpListBatch)
		}
	}
	if len(batch) > 0 {
		return emit(batch)
	}
	return nil
}

func (b *sftpBackend) stat(_ context.Context, target string) (fileInfo, error) {
	info, err := b.client.Stat(target)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return fileInfo{}, nil
		}
		return fileInfo{}, err
	}
	return fileInfo{Exists: true, Dir: info.IsDir(), Size: info.Size()}, nil
}

func (b *sftpBackend) openRead(_ context.Context, target string, offset int64) (io.ReadCloser, int64, bool, error) {
	f, err := b.client.Open(target)
	if err != nil {
		return nil, -1, false, err
	}
	size := int64(-1)
	if info, serr := f.Stat(); serr == nil {
		size = info.Size()
	}
	if offset > 0 {
		if _, err := f.Seek(offset, io.SeekStart); err != nil {
			_ = f.Close()
			return nil, -1, false, err
		}
	}
	return f, size, offset > 0, nil
}

type sftpWriter struct {
	b    *sftpBackend
	f    *sftp.File
	path string

	closeOnce sync.Once
	abortOnce sync.Once
	err       error
}

func (w *sftpWriter) Write(p []byte) (int, error) { return w.f.Write(p) }

func (w *sftpWriter) Close() error {
	w.closeOnce.Do(func() { w.err = w.f.Close() })
	return w.err
}

func (w *sftpWriter) Abort() {
	w.closeOnce.Do(func() { w.err = w.f.Close() })
	w.abortOnce.Do(func() {
		_ = w.b.client.Remove(w.path) // do not leave a half file behind
	})
}

func (b *sftpBackend) openWrite(_ context.Context, target string) (writeStream, error) {
	f, err := b.client.OpenFile(target, os.O_WRONLY|os.O_CREATE|os.O_TRUNC)
	if err != nil {
		return nil, err
	}
	return &sftpWriter{b: b, f: f, path: target}, nil
}

func (b *sftpBackend) mkdir(_ context.Context, target string) error {
	return b.client.MkdirAll(strings.TrimRight(target, "/"))
}

func (b *sftpBackend) remove(ctx context.Context, target string, dir bool) error {
	if !dir {
		return b.client.Remove(target)
	}
	return b.removeAll(ctx, strings.TrimRight(target, "/"), 0)
}

// removeAll deletes a folder and everything in it. Links are removed as links; they are never followed.
func (b *sftpBackend) removeAll(ctx context.Context, folder string, depth int) error {
	if depth > 64 {
		return errors.New("the folder is nested too deeply")
	}
	infos, err := b.client.ReadDir(folder)
	if err != nil {
		return err
	}
	for _, info := range infos {
		if err := ctx.Err(); err != nil {
			return err
		}
		child := path.Join(folder, info.Name())
		if info.IsDir() && info.Mode()&os.ModeSymlink == 0 {
			if err := b.removeAll(ctx, child, depth+1); err != nil {
				return err
			}
			continue
		}
		if err := b.client.Remove(child); err != nil {
			return err
		}
	}
	return b.client.RemoveDirectory(folder)
}

func (b *sftpBackend) rename(_ context.Context, from, to string) error {
	if err := b.client.PosixRename(from, to); err == nil {
		return nil
	}
	return b.client.Rename(from, to)
}

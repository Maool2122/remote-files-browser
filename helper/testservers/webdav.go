package testservers

import (
	"context"
	"net"
	"net/http"
	"os"
	"path"
	"strconv"
	"strings"

	"golang.org/x/net/webdav"
)

// WebDAV is a running WebDAV server. It is the WebDAV package of the Go project (not code of ours), with the files in memory,
// and it asks for the user name and password (basic login) like a real server does.
type WebDAV struct {
	Host string
	Port int

	fs  webdav.FileSystem
	srv *http.Server
}

// StartWebDAV starts a WebDAV server on 127.0.0.1 on a free port.
func StartWebDAV() (*WebDAV, error) {
	fs := webdav.NewMemFS()
	dav := &webdav.Handler{FileSystem: fs, LockSystem: webdav.NewMemLS()}
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		user, pass, ok := r.BasicAuth()
		if !ok || user != User || pass != Password {
			w.Header().Set("WWW-Authenticate", `Basic realm="Rocuo test server"`)
			http.Error(w, "Sign in", http.StatusUnauthorized)
			return
		}
		dav.ServeHTTP(w, r)
	})
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return nil, err
	}
	s := &WebDAV{Host: "127.0.0.1", Port: ln.Addr().(*net.TCPAddr).Port, fs: fs, srv: &http.Server{Handler: handler}}
	go func() { _ = s.srv.Serve(ln) }()
	return s, nil
}

// Seed makes a file, and the folders above it.
func (s *WebDAV) Seed(name string, data []byte) error {
	ctx := context.Background()
	current := ""
	for _, part := range strings.Split(strings.Trim(path.Dir(name), "/"), "/") {
		if part == "" || part == "." {
			continue
		}
		current += "/" + part
		if err := s.fs.Mkdir(ctx, current, 0o755); err != nil && !os.IsExist(err) {
			return err
		}
	}
	f, err := s.fs.OpenFile(ctx, name, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o644)
	if err != nil {
		return err
	}
	if _, err := f.Write(data); err != nil {
		_ = f.Close()
		return err
	}
	return f.Close()
}

// URL is the address of the server.
func (s *WebDAV) URL() string { return "http://" + s.Host + ":" + strconv.Itoa(s.Port) + "/" }

// Close stops the server.
func (s *WebDAV) Close() { _ = s.srv.Close() }

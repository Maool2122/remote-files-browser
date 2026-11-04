package main

import (
	"context"
	"io"
)

// entry is one line of a folder listing, in the short form the extension reads.
type entry struct {
	Name     string `json:"n"`
	Dir      bool   `json:"d"`
	Size     int64  `json:"s"`           // -1 when the server does not say
	Modified int64  `json:"t"`           // milliseconds since 1970, 0 when the server does not say
	Mode     string `json:"m,omitempty"` // "rwxr-xr-x" when the server says
	Link     bool   `json:"l,omitempty"`
}

// fileInfo is the answer of "stat".
type fileInfo struct {
	Exists bool
	Dir    bool
	Size   int64
}

// writeStream takes the bytes of one upload. Close finishes it (and returns the error of the server);
// Abort throws the upload away and removes the part that was already written.
type writeStream interface {
	io.Writer
	Close() error
	Abort()
}

// backend is one open connection to a server (FTP, FTPS or SFTP). All paths are absolute and use "/".
// Every method works on the connection it needs, so several calls can run at the same time.
type backend interface {
	// home is the folder the person lands in after login.
	home() string
	// secure says if the link is encrypted; note is a short sentence for the toolbar.
	secure() (bool, string)

	list(ctx context.Context, path string, emit func([]entry) error) error
	stat(ctx context.Context, path string) (fileInfo, error)
	// openRead starts a download at `offset`. `resumed` is false when the server could not skip the bytes
	// (then the stream starts at 0 and the caller must throw the first `offset` bytes away itself).
	openRead(ctx context.Context, path string, offset int64) (rc io.ReadCloser, size int64, resumed bool, err error)
	openWrite(ctx context.Context, path string) (writeStream, error)
	mkdir(ctx context.Context, path string) error
	remove(ctx context.Context, path string, dir bool) error
	rename(ctx context.Context, from, to string) error
	close()
}

// connectConfig is what the extension sends with "open".
type connectConfig struct {
	Kind     string
	Host     string
	Port     int
	User     string
	Password string
	KeyPath  string
	TLS      string
	NoEPSV   bool
	Trusted  []string
}

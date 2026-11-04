package main

import (
	"context"
	"crypto/tls"
	"errors"
	"io"
	"net"
	"net/textproto"
	"path"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/jlaffaye/ftp"
)

const (
	ftpMaxConnections = 6
	ftpDialTimeout    = 20 * time.Second
	ftpStallTimeout   = 90 * time.Second
	ftpListBatch      = 500
)

// ---------------------------------------------------------------------------------------------------------------------
// One FTP connection is one control socket plus one data socket per transfer. The library hides its sockets, so every
// socket is opened through ftpDialer: that way a cancelled transfer can close them all at once.

type ftpDialer struct {
	mode   string // "plain", "explicit" (AUTH TLS) or "implicit" (TLS from the first byte)
	config *tls.Config

	mu    sync.Mutex
	calls int
	live  map[*trackedConn]struct{}
	dead  bool
}

type trackedConn struct {
	net.Conn
	owner *ftpDialer
	once  sync.Once
}

func (t *trackedConn) Close() error {
	t.once.Do(func() {
		t.owner.mu.Lock()
		delete(t.owner.live, t)
		t.owner.mu.Unlock()
	})
	return t.Conn.Close()
}

func (d *ftpDialer) dial(network, address string) (net.Conn, error) {
	d.mu.Lock()
	if d.dead {
		d.mu.Unlock()
		return nil, net.ErrClosed
	}
	first := d.calls == 0
	d.calls++
	d.mu.Unlock()

	dialer := net.Dialer{Timeout: ftpDialTimeout, KeepAlive: 30 * time.Second}
	raw, err := dialer.Dial(network, address)
	if err != nil {
		return nil, err
	}
	conn := &trackedConn{Conn: raw, owner: d}
	d.mu.Lock()
	if d.dead {
		d.mu.Unlock()
		raw.Close()
		return nil, net.ErrClosed
	}
	if d.live == nil {
		d.live = map[*trackedConn]struct{}{}
	}
	d.live[conn] = struct{}{}
	d.mu.Unlock()

	// The first socket is the control connection. With explicit TLS the library upgrades it itself after AUTH TLS;
	// every data socket is encrypted when a TLS setting is on.
	if d.config != nil && (d.mode == "implicit" || !first) {
		return &tlsConn{Conn: tls.Client(conn, d.config)}, nil
	}
	return conn, nil
}

// tlsConn is an encrypted FTP socket that closes politely. With TLS 1.3 a server sends session tickets right after the
// handshake. A client that only uploads never reads them, and closing a socket that still holds unread bytes ends with
// a reset instead of a normal close: the server then thinks the upload broke off. So before closing, say goodbye
// (close_notify) and read what the server still has to say.
type tlsConn struct {
	*tls.Conn
}

func (c *tlsConn) Close() error {
	if c.Conn.ConnectionState().HandshakeComplete {
		_ = c.Conn.SetWriteDeadline(time.Now().Add(3 * time.Second))
		if c.Conn.CloseWrite() == nil {
			_ = c.Conn.SetReadDeadline(time.Now().Add(3 * time.Second))
			_, _ = io.Copy(io.Discard, c.Conn)
		}
	}
	return c.Conn.Close()
}

func (d *ftpDialer) isDead() bool {
	d.mu.Lock()
	defer d.mu.Unlock()
	return d.dead
}

// abort closes every socket of the connection. Whatever is reading or writing on them stops with an error.
func (d *ftpDialer) abort() {
	d.mu.Lock()
	d.dead = true
	sockets := make([]*trackedConn, 0, len(d.live))
	for conn := range d.live {
		sockets = append(sockets, conn)
	}
	d.mu.Unlock()
	for _, conn := range sockets {
		conn.Close()
	}
}

type ftpConn struct {
	sc   *ftp.ServerConn
	d    *ftpDialer
	last time.Time
}

func (c *ftpConn) quit() {
	done := make(chan struct{})
	go func() {
		_ = c.sc.Quit()
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(2 * time.Second):
	}
	c.d.abort()
}

// ---------------------------------------------------------------------------------------------------------------------

type ftpBackend struct {
	cfg      connectConfig
	trust    *trustHolder
	tlsCfg   *tls.Config
	mode     string
	homeDir  string
	protect  bool
	protNote string

	mu      sync.Mutex
	idle    []*ftpConn
	busy    int
	opening int
	limit   int
	closed  bool
	wake    chan struct{}
}

func openFTP(ctx context.Context, cfg connectConfig) (backend, error) {
	b := &ftpBackend{cfg: cfg, trust: &trustHolder{}, limit: ftpMaxConnections, wake: make(chan struct{})}
	b.tlsCfg = tlsConfig(cfg.Host, cfg.Trusted, b.trust)

	var modes []string
	switch cfg.TLS {
	case "implicit":
		modes = []string{"implicit"}
	case "explicit":
		modes = []string{"explicit"}
	case "none":
		modes = []string{"plain"}
	default: // "auto": use TLS when the server offers it
		modes = []string{"explicit", "plain"}
	}

	var lastErr error
	for i, mode := range modes {
		conn, err := b.dialOne(ctx, mode)
		if err == nil {
			b.mode = mode
			b.protect = mode != "plain"
			switch mode {
			case "plain":
				b.protNote = "Not encrypted"
			default:
				b.protNote = "Encrypted (TLS)"
			}
			if dir, derr := conn.sc.CurrentDir(); derr == nil && dir != "" {
				b.homeDir = dir
			} else {
				b.homeDir = "/"
			}
			b.mu.Lock()
			b.idle = append(b.idle, conn)
			b.mu.Unlock()
			return b, nil
		}
		lastErr = err
		var refused *tlsRefused
		if i+1 < len(modes) && errors.As(err, &refused) {
			continue // the server does not offer TLS: try without
		}
		break
	}
	return nil, lastErr
}

// tlsRefused means the server answered AUTH TLS with an error: it does not do TLS.
type tlsRefused struct{ err error }

func (e *tlsRefused) Error() string { return e.err.Error() }
func (e *tlsRefused) Unwrap() error { return e.err }

func (b *ftpBackend) dialOne(ctx context.Context, mode string) (*ftpConn, error) {
	b.trust.set(nil)
	d := &ftpDialer{mode: mode}
	options := []ftp.DialOption{
		ftp.DialWithDialFunc(d.dial),
		ftp.DialWithShutTimeout(30 * time.Second),
		ftp.DialWithDisabledEPSV(b.cfg.NoEPSV),
	}
	switch mode {
	case "explicit":
		d.config = b.tlsCfg
		options = append(options, ftp.DialWithExplicitTLS(b.tlsCfg))
	case "implicit":
		d.config = b.tlsCfg
		options = append(options, ftp.DialWithTLS(b.tlsCfg))
	}
	address := net.JoinHostPort(b.cfg.Host, strconv.Itoa(b.cfg.Port))

	type result struct {
		sc  *ftp.ServerConn
		err error
	}
	done := make(chan result, 1)
	go func() {
		sc, err := ftp.Dial(address, options...)
		if err != nil {
			var te *textproto.Error
			if mode == "explicit" && errors.As(err, &te) && te.Code >= 500 {
				err = &tlsRefused{err: err}
			}
			done <- result{nil, err}
			return
		}
		if err := b.login(sc); err != nil {
			go func() { _ = sc.Quit(); d.abort() }()
			done <- result{nil, err}
			return
		}
		done <- result{sc, nil}
	}()

	select {
	case r := <-done:
		if r.err != nil {
			d.abort()
			if problem := b.trust.get(); problem != nil {
				return nil, problem
			}
			return nil, r.err
		}
		return &ftpConn{sc: r.sc, d: d, last: time.Now()}, nil
	case <-ctx.Done():
		d.abort()
		<-done
		return nil, ctx.Err()
	}
}

func (b *ftpBackend) login(sc *ftp.ServerConn) error {
	user, password := b.cfg.User, b.cfg.Password
	if user == "" {
		user, password = "anonymous", "anonymous@"
	}
	err := sc.Login(user, password)
	if err == nil {
		return nil
	}
	var te *textproto.Error
	var ne net.Error
	switch {
	case errors.As(err, &te):
		return err
	case errors.As(err, &ne), errors.Is(err, io.EOF), errors.Is(err, io.ErrUnexpectedEOF):
		return err
	}
	if _, isTrust := err.(*trustError); isTrust {
		return err
	}
	// The library returns a plain error with the server's words when USER itself is refused.
	return &opError{Code: "auth", Message: "The server did not accept the user name or password (" + cleanFTPText(err.Error()) + ")."}
}

func (b *ftpBackend) home() string { return b.homeDir }

func (b *ftpBackend) secure() (bool, string) { return b.protect, b.protNote }

func (b *ftpBackend) notify() {
	close(b.wake)
	b.wake = make(chan struct{})
}

// acquire gives a connection that nobody else uses: an idle one, or a new one while the server lets us have more.
func (b *ftpBackend) acquire(ctx context.Context) (*ftpConn, error) {
	for attempt := 0; ; attempt++ {
		b.mu.Lock()
		if b.closed {
			b.mu.Unlock()
			return nil, newError("session", "The connection was closed.")
		}
		if n := len(b.idle); n > 0 {
			c := b.idle[n-1]
			b.idle = b.idle[:n-1]
			b.busy++
			b.mu.Unlock()
			if c.d.isDead() || (time.Since(c.last) > 15*time.Second && c.sc.NoOp() != nil) {
				b.release(c, true) // the server dropped the idle link: use another one
				continue
			}
			return c, nil
		}
		if b.busy+b.opening < b.limit {
			b.opening++
			b.mu.Unlock()
			c, err := b.dialOne(ctx, b.mode)
			b.mu.Lock()
			b.opening--
			if err != nil {
				var te *textproto.Error
				if errors.As(err, &te) && te.Code == 421 && b.busy > 0 && attempt < 10 {
					// "Too many connections": stay under what the server allows and wait for a free one.
					b.limit = b.busy
					b.notify()
					b.mu.Unlock()
					continue
				}
				b.notify()
				b.mu.Unlock()
				return nil, err
			}
			b.busy++
			b.mu.Unlock()
			return c, nil
		}
		wait := b.wake
		b.mu.Unlock()
		select {
		case <-wait:
		case <-ctx.Done():
			return nil, ctx.Err()
		}
	}
}

// release gives a connection back. A broken one (or one in an unknown state, after a cancelled transfer) is closed.
func (b *ftpBackend) release(c *ftpConn, broken bool) {
	b.mu.Lock()
	b.busy--
	if broken || b.closed {
		b.notify()
		b.mu.Unlock()
		go c.quit()
		return
	}
	c.last = time.Now()
	b.idle = append(b.idle, c)
	b.notify()
	b.mu.Unlock()
}

// with runs one piece of work on a connection of its own. Cancelling ctx closes the sockets, which ends the work.
func (b *ftpBackend) with(ctx context.Context, work func(c *ftpConn) error) error {
	c, err := b.acquire(ctx)
	if err != nil {
		return err
	}
	stop := context.AfterFunc(ctx, c.d.abort)
	err = work(c)
	cancelled := !stop()
	b.release(c, cancelled || isBroken(err))
	if cancelled && ctx.Err() != nil {
		return ctx.Err()
	}
	return err
}

// isBroken tells whether the connection can no longer be trusted after this error. A refusal such as "550 no such file"
// leaves it fine; a lost socket or a timeout does not.
func isBroken(err error) bool {
	if err == nil {
		return false
	}
	var te *textproto.Error
	if errors.As(err, &te) {
		return te.Code == 421
	}
	return true
}

func (b *ftpBackend) close() {
	b.mu.Lock()
	b.closed = true
	idle := b.idle
	b.idle = nil
	b.notify()
	b.mu.Unlock()
	for _, c := range idle {
		c.quit()
	}
}

func toEntry(e *ftp.Entry) entry {
	out := entry{Name: e.Name, Size: int64(e.Size)}
	switch e.Type {
	case ftp.EntryTypeFolder:
		out.Dir = true
	case ftp.EntryTypeLink:
		out.Link = true
	}
	if !e.Time.IsZero() {
		out.Modified = e.Time.UnixMilli()
	}
	return out
}

func (b *ftpBackend) list(ctx context.Context, folder string, emit func([]entry) error) error {
	var items []entry
	err := b.with(ctx, func(c *ftpConn) error {
		listing, err := c.sc.List(folder)
		if err != nil && len(listing) == 0 {
			return err
		}
		// A "550 empty" or an error at the very end after real entries still gives what was read.
		probes := 0
		for _, e := range listing {
			if e.Name == "." || e.Name == ".." || e.Name == "" || strings.ContainsAny(e.Name, "/\x00") {
				continue
			}
			item := toEntry(e)
			if item.Link && probes < 300 {
				// A link can point to a folder or a file; the listing does not say which.
				probes++
				if c.sc.ChangeDir(joinPath(folder, e.Name)) == nil {
					item.Dir = true
					item.Size = -1
				}
			}
			items = append(items, item)
		}
		return nil
	})
	if err != nil {
		return err
	}
	for start := 0; start < len(items); start += ftpListBatch {
		end := min(start+ftpListBatch, len(items))
		if err := emit(items[start:end]); err != nil {
			return err
		}
	}
	return nil
}

func joinPath(folder, name string) string {
	if strings.HasSuffix(folder, "/") {
		return folder + name
	}
	return folder + "/" + name
}

func (b *ftpBackend) stat(ctx context.Context, target string) (fileInfo, error) {
	var info fileInfo
	err := b.with(ctx, func(c *ftpConn) error {
		if c.sc.IsTimePreciseInList() { // the server knows MLST: one exact answer
			e, err := c.sc.GetEntry(target)
			if err == nil && e != nil {
				info = fileInfo{Exists: true, Dir: e.Type == ftp.EntryTypeFolder, Size: int64(e.Size)}
				return nil
			}
			var te *textproto.Error
			if errors.As(err, &te) && te.Code == 550 {
				return nil
			}
		}
		if size, err := c.sc.FileSize(target); err == nil {
			info = fileInfo{Exists: true, Size: size}
			return nil
		}
		if err := c.sc.ChangeDir(target); err == nil {
			info = fileInfo{Exists: true, Dir: true, Size: -1}
		}
		return nil
	})
	return info, err
}

// ftpReader is the data socket of one download.
type ftpReader struct {
	b     *ftpBackend
	c     *ftpConn
	resp  *ftp.Response
	eof   bool
	once  sync.Once
	final error
}

func (r *ftpReader) Read(p []byte) (int, error) {
	_ = r.resp.SetDeadline(time.Now().Add(ftpStallTimeout))
	n, err := r.resp.Read(p)
	if err == io.EOF {
		r.eof = true
	}
	return n, err
}

func (r *ftpReader) Close() error {
	r.once.Do(func() {
		if !r.eof {
			// Stopped early (cancelled, or the caller failed): the connection is left in an unknown state.
			r.c.d.abort()
			_ = r.resp.Close()
			r.b.release(r.c, true)
			return
		}
		r.final = r.resp.Close()
		r.b.release(r.c, r.final != nil)
	})
	return r.final
}

func (b *ftpBackend) openRead(ctx context.Context, target string, offset int64) (io.ReadCloser, int64, bool, error) {
	c, err := b.acquire(ctx)
	if err != nil {
		return nil, -1, false, err
	}
	size := int64(-1)
	if n, serr := c.sc.FileSize(target); serr == nil {
		size = n
	}
	resumed := offset > 0
	resp, err := c.sc.RetrFrom(target, uint64(offset))
	if err != nil && offset > 0 {
		var te *textproto.Error
		if errors.As(err, &te) && te.Code >= 500 && te.Code != 550 {
			// The server cannot start in the middle: send the whole file and let the caller skip the first part.
			resumed = false
			resp, err = c.sc.RetrFrom(target, 0)
		}
	}
	if err != nil {
		b.release(c, isBroken(err))
		return nil, -1, false, err
	}
	return &ftpReader{b: b, c: c, resp: resp}, size, resumed, nil
}

// ftpWriter is one upload: the bytes go through a pipe into STOR, which runs on its own goroutine.
type ftpWriter struct {
	b    *ftpBackend
	c    *ftpConn
	path string
	pw   *io.PipeWriter
	done chan error

	closeOnce sync.Once
	abortOnce sync.Once
	err       error
}

// Write blocks until the STOR goroutine has taken the bytes, so a slow server slows the sender down.
func (w *ftpWriter) Write(p []byte) (int, error) {
	return w.pw.Write(p)
}

// Close ends the upload and returns what the server said about it.
func (w *ftpWriter) Close() error {
	w.closeOnce.Do(func() {
		_ = w.pw.Close()
		w.err = <-w.done
		w.b.release(w.c, isBroken(w.err))
	})
	return w.err
}

// Abort throws the upload away. It may be called from another goroutine while a Write or Close is stuck on the network.
func (w *ftpWriter) Abort() {
	w.c.d.abort()
	w.closeOnce.Do(func() {
		_ = w.pw.CloseWithError(context.Canceled)
		w.err = <-w.done
		w.b.release(w.c, true)
	})
	w.abortOnce.Do(func() {
		ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		defer cancel()
		_ = w.b.remove(ctx, w.path, false) // do not leave a half file behind
	})
}

func (b *ftpBackend) openWrite(ctx context.Context, target string) (writeStream, error) {
	c, err := b.acquire(ctx)
	if err != nil {
		return nil, err
	}
	pr, pw := io.Pipe()
	done := make(chan error, 1)
	go func() {
		err := c.sc.Stor(target, pr)
		_ = pr.CloseWithError(err) // a sender that is still writing gets this error (nil closes the pipe)
		done <- err
	}()
	return &ftpWriter{b: b, c: c, path: target, pw: pw, done: done}, nil
}

func (b *ftpBackend) mkdir(ctx context.Context, target string) error {
	return b.with(ctx, func(c *ftpConn) error { return ftpMkdirAll(c, target, 0) })
}

func ftpMkdirAll(c *ftpConn, target string, depth int) error {
	target = strings.TrimRight(target, "/")
	if target == "" {
		return nil
	}
	err := c.sc.MakeDir(target)
	if err == nil {
		return nil
	}
	if c.sc.ChangeDir(target) == nil {
		return nil // it exists already
	}
	var te *textproto.Error
	if errors.As(err, &te) && te.Code < 500 {
		return err
	}
	parent := path.Dir(target)
	if depth < 32 && parent != "/" && parent != "." && parent != target {
		if perr := ftpMkdirAll(c, parent, depth+1); perr != nil {
			return perr
		}
		return c.sc.MakeDir(target)
	}
	return err
}

func (b *ftpBackend) remove(ctx context.Context, target string, dir bool) error {
	return b.with(ctx, func(c *ftpConn) error {
		if !dir {
			return c.sc.Delete(target)
		}
		return ftpRemoveAll(ctx, c, strings.TrimRight(target, "/"), 0)
	})
}

func ftpRemoveAll(ctx context.Context, c *ftpConn, folder string, depth int) error {
	if depth > 64 {
		return errors.New("the folder is nested too deeply")
	}
	listing, err := c.sc.List(folder)
	if err != nil && len(listing) == 0 {
		var te *textproto.Error
		if !(errors.As(err, &te) && te.Code == 550) {
			return err
		}
	}
	for _, e := range listing {
		if err := ctx.Err(); err != nil {
			return err
		}
		if e.Name == "." || e.Name == ".." || e.Name == "" {
			continue
		}
		child := joinPath(folder, e.Name)
		if e.Type == ftp.EntryTypeFolder {
			if err := ftpRemoveAll(ctx, c, child, depth+1); err != nil {
				return err
			}
			continue
		}
		if err := c.sc.Delete(child); err != nil {
			return err
		}
	}
	return c.sc.RemoveDir(folder)
}

func (b *ftpBackend) rename(ctx context.Context, from, to string) error {
	return b.with(ctx, func(c *ftpConn) error { return c.sc.Rename(from, to) })
}

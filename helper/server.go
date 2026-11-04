package main

import (
	"bufio"
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"runtime"
	"sync"
	"time"
)

const (
	version         = "1.0.0"
	protocolVersion = 1

	// One "data" message carries at most this many bytes of a download (before base64). Chrome accepts 1 MB per message.
	readChunk = 192 * 1024
)

// operation is one request that is still running. The extension can talk to it afterwards:
// "credit" lets a download send more, "chunk" and "end" feed an upload, "cancel" stops it.
type operation struct {
	cancel context.CancelFunc
	credit chan struct{}
	chunks chan chunkMessage
}

type chunkMessage struct {
	data []byte
	end  bool
}

type server struct {
	out    *writer
	ctx    context.Context
	cancel context.CancelFunc
	wg     sync.WaitGroup

	mu       sync.Mutex
	sessions map[string]backend
	ops      map[int64]*operation
}

func newServer(out io.Writer) *server {
	ctx, cancel := context.WithCancel(context.Background())
	return &server{
		out:      newWriter(out),
		ctx:      ctx,
		cancel:   cancel,
		sessions: map[string]backend{},
		ops:      map[int64]*operation{},
	}
}

// run reads requests until Chrome closes the pipe (the extension page was closed, or the browser quit).
func (s *server) run(in io.Reader) error {
	reader := bufio.NewReaderSize(in, 256*1024)
	for {
		raw, err := readMessage(reader)
		if err != nil {
			s.shutdown()
			if errors.Is(err, io.EOF) || errors.Is(err, io.ErrUnexpectedEOF) {
				return nil
			}
			return err
		}
		var m message
		if err := json.Unmarshal(raw, &m); err != nil {
			continue
		}
		s.dispatch(&m)
	}
}

func (s *server) shutdown() {
	s.cancel()
	done := make(chan struct{})
	go func() {
		s.wg.Wait()
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(3 * time.Second):
	}
	s.mu.Lock()
	sessions := s.sessions
	s.sessions = map[string]backend{}
	s.mu.Unlock()
	for _, b := range sessions {
		b.close()
	}
}

func (s *server) dispatch(m *message) {
	switch m.Op {
	case "hello":
		s.ok(m.ID, map[string]any{"version": version, "protocol": protocolVersion, "os": runtime.GOOS, "arch": runtime.GOARCH})
	case "credit":
		if op := s.operation(m.Target); op != nil {
			for i := 0; i < max(1, m.N); i++ {
				select {
				case op.credit <- struct{}{}:
				default:
				}
			}
		}
	case "chunk":
		if op := s.operation(m.Target); op != nil {
			data, err := base64.StdEncoding.DecodeString(m.B64)
			if err != nil {
				op.cancel()
				return
			}
			select {
			case op.chunks <- chunkMessage{data: data}:
			case <-s.ctx.Done():
			}
		}
	case "end":
		if op := s.operation(m.Target); op != nil {
			select {
			case op.chunks <- chunkMessage{end: true}:
			case <-s.ctx.Done():
			}
		}
	case "cancel":
		if op := s.operation(m.Target); op != nil {
			op.cancel()
		}
	default:
		s.start(m)
	}
}

func (s *server) operation(id int64) *operation {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.ops[id]
}

func (s *server) start(m *message) {
	ctx, cancel := context.WithCancel(s.ctx)
	op := &operation{cancel: cancel, credit: make(chan struct{}, 1<<16), chunks: make(chan chunkMessage, 64)}
	s.mu.Lock()
	s.ops[m.ID] = op
	s.mu.Unlock()
	s.wg.Add(1)
	go func() {
		defer s.wg.Done()
		defer func() {
			s.mu.Lock()
			delete(s.ops, m.ID)
			s.mu.Unlock()
			cancel()
		}()
		defer func() {
			if r := recover(); r != nil {
				s.fail(m.ID, fmt.Errorf("internal error: %v", r))
			}
		}()
		result, err := s.handle(ctx, m, op)
		if err != nil {
			s.fail(m.ID, err)
			return
		}
		s.ok(m.ID, result)
	}()
}

func (s *server) ok(id int64, result map[string]any) {
	reply := map[string]any{"id": id, "ok": true}
	for k, v := range result {
		reply[k] = v
	}
	s.send(reply)
}

func (s *server) fail(id int64, err error) {
	s.send(map[string]any{"id": id, "ok": false, "error": wire(err)})
}

func (s *server) event(id int64, name string, extra map[string]any) error {
	event := map[string]any{"id": id, "event": name}
	for k, v := range extra {
		event[k] = v
	}
	return s.send(event)
}

func (s *server) send(value any) error {
	err := s.out.send(value)
	if err != nil {
		s.cancel() // Chrome is gone: stop everything
	}
	return err
}

func (s *server) session(id string) (backend, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	b, ok := s.sessions[id]
	if !ok {
		return nil, newError("session", "The connection to the server is not open.")
	}
	return b, nil
}

func (s *server) handle(ctx context.Context, m *message, op *operation) (map[string]any, error) {
	switch m.Op {
	case "open":
		return s.opOpen(ctx, m)
	case "close":
		s.mu.Lock()
		b := s.sessions[m.Session]
		delete(s.sessions, m.Session)
		s.mu.Unlock()
		if b != nil {
			b.close()
		}
		return nil, nil
	}

	b, err := s.session(m.Session)
	if err != nil {
		return nil, err
	}
	switch m.Op {
	case "list":
		return nil, b.list(ctx, m.Path, func(items []entry) error {
			return s.event(m.ID, "entries", map[string]any{"entries": items})
		})
	case "stat":
		info, err := b.stat(ctx, m.Path)
		if err != nil {
			return nil, err
		}
		return map[string]any{"exists": info.Exists, "dir": info.Dir, "size": info.Size}, nil
	case "read":
		return s.opRead(ctx, m, op, b)
	case "write":
		return s.opWrite(ctx, m, op, b)
	case "mkdir":
		return nil, b.mkdir(ctx, m.Path)
	case "remove":
		return nil, b.remove(ctx, m.Path, m.Dir)
	case "rename":
		return nil, b.rename(ctx, m.From, m.To)
	}
	return nil, newError("generic", "The helper does not know the request \"%s\". Update the helper.", m.Op)
}

func (s *server) opOpen(ctx context.Context, m *message) (map[string]any, error) {
	cfg := connectConfig{
		Kind: m.Kind, Host: m.Host, Port: m.Port, User: m.User, Password: m.Password,
		KeyPath: m.KeyPath, TLS: m.TLS, NoEPSV: m.NoEPSV, Trusted: m.Trusted,
	}
	if cfg.Host == "" {
		return nil, newError("generic", "There is no server address.")
	}
	var b backend
	var err error
	switch cfg.Kind {
	case "ftp", "ftps":
		if cfg.TLS == "" {
			cfg.TLS = map[string]string{"ftp": "auto", "ftps": "implicit"}[cfg.Kind]
		}
		if cfg.Port == 0 {
			cfg.Port = map[string]int{"auto": 21, "explicit": 21, "none": 21, "implicit": 990}[cfg.TLS]
		}
		b, err = openFTP(ctx, cfg)
	case "sftp":
		if cfg.Port == 0 {
			cfg.Port = 22
		}
		b, err = openSFTP(ctx, cfg)
	default:
		return nil, newError("generic", "The helper does not do \"%s\" connections.", cfg.Kind)
	}
	if err != nil {
		return nil, err
	}

	s.mu.Lock()
	old := s.sessions[m.Session]
	s.sessions[m.Session] = b
	s.mu.Unlock()
	if old != nil {
		old.close()
	}
	secure, note := b.secure()
	return map[string]any{"secure": secure, "note": note, "home": b.home()}, nil
}

// chunkWriter cuts a download into messages. It sends one only when the extension has given a credit for it, so a slow
// page never makes the helper pile up data in memory.
type chunkWriter struct {
	s     *server
	id    int64
	ctx   context.Context
	op    *operation
	buf   []byte
	skip  int64
	total int64
}

func (c *chunkWriter) Write(p []byte) (int, error) {
	n := len(p)
	if c.skip > 0 {
		drop := min(int64(len(p)), c.skip)
		p = p[drop:]
		c.skip -= drop
	}
	for len(p) > 0 {
		take := min(readChunk-len(c.buf), len(p))
		c.buf = append(c.buf, p[:take]...)
		p = p[take:]
		if len(c.buf) >= readChunk {
			if err := c.flush(); err != nil {
				return 0, err
			}
		}
	}
	return n, nil
}

func (c *chunkWriter) flush() error {
	if len(c.buf) == 0 {
		return nil
	}
	select {
	case <-c.op.credit:
	case <-c.ctx.Done():
		return c.ctx.Err()
	}
	data := base64.StdEncoding.EncodeToString(c.buf)
	c.total += int64(len(c.buf))
	c.buf = c.buf[:0]
	return c.s.event(c.id, "data", map[string]any{"b64": data})
}

func (s *server) opRead(ctx context.Context, m *message, op *operation, b backend) (map[string]any, error) {
	rc, size, resumed, err := b.openRead(ctx, m.Path, m.Offset)
	if err != nil {
		return nil, err
	}
	// Cancelling must also free a read that is stuck on the network.
	stop := context.AfterFunc(ctx, func() { _ = rc.Close() })
	defer stop()

	if err := s.event(m.ID, "meta", map[string]any{"size": size, "resumed": resumed}); err != nil {
		_ = rc.Close()
		return nil, err
	}
	credit := m.Credit
	if credit <= 0 {
		credit = 8
	}
	for i := 0; i < credit; i++ {
		op.credit <- struct{}{}
	}
	writer := &chunkWriter{s: s, id: m.ID, ctx: ctx, op: op}
	if m.Offset > 0 && !resumed {
		writer.skip = m.Offset
	}
	_, copyErr := io.CopyBuffer(writer, rc, make([]byte, 256*1024))
	if copyErr == nil {
		copyErr = writer.flush()
	}
	closeErr := rc.Close()
	switch {
	case ctx.Err() != nil:
		return nil, ctx.Err()
	case copyErr != nil:
		return nil, copyErr
	case closeErr != nil:
		return nil, closeErr // the server ended the transfer with an error: the file may be cut short
	}
	return map[string]any{"bytes": writer.total}, nil
}

func (s *server) opWrite(ctx context.Context, m *message, op *operation, b backend) (map[string]any, error) {
	w, err := b.openWrite(ctx, m.Path)
	if err != nil {
		return nil, err
	}
	// Cancelling must also free a write that is stuck on the network.
	stop := context.AfterFunc(ctx, w.Abort)
	defer stop()

	if err := s.event(m.ID, "ready", nil); err != nil {
		w.Abort()
		return nil, err
	}
	var total int64
	for {
		select {
		case c := <-op.chunks:
			if c.end {
				if err := w.Close(); err != nil {
					w.Abort()
					return nil, err
				}
				return map[string]any{"bytes": total}, nil
			}
			if _, err := w.Write(c.data); err != nil {
				w.Abort()
				if ctx.Err() != nil {
					return nil, ctx.Err()
				}
				return nil, err
			}
			total += int64(len(c.data))
			if err := s.event(m.ID, "ack", map[string]any{"n": 1, "bytes": len(c.data)}); err != nil {
				w.Abort()
				return nil, err
			}
		case <-ctx.Done():
			w.Abort()
			return nil, ctx.Err()
		}
	}
}

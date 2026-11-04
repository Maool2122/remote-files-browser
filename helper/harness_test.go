package main

import (
	"bufio"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"sync"
	"testing"
	"time"
)

// A pretend extension: it talks to the helper through pipes with the real wire format.

type reply map[string]any

func (r reply) ok() bool { return r["ok"] == true }

func (r reply) errCode() string {
	if e, ok := r["error"].(map[string]any); ok {
		s, _ := e["code"].(string)
		return s
	}
	return ""
}

func (r reply) errMessage() string {
	if e, ok := r["error"].(map[string]any); ok {
		s, _ := e["message"].(string)
		return s
	}
	return ""
}

func (r reply) errField(name string) string {
	if e, ok := r["error"].(map[string]any); ok {
		s, _ := e[name].(string)
		return s
	}
	return ""
}

func (r reply) str(name string) string {
	s, _ := r[name].(string)
	return s
}

func (r reply) num(name string) int64 {
	f, _ := r[name].(float64)
	return int64(f)
}

type call struct {
	id     int64
	events chan reply
	done   chan reply
}

type harness struct {
	t      *testing.T
	srv    *server
	toSrv  *io.PipeWriter
	out    *writer
	mu     sync.Mutex
	nextID int64
	calls  map[int64]*call
	closed chan struct{}
}

func newHarness(t *testing.T) *harness {
	t.Helper()
	inR, inW := io.Pipe()
	outR, outW := io.Pipe()
	h := &harness{t: t, srv: newServer(outW), toSrv: inW, out: newWriter(inW), calls: map[int64]*call{}, closed: make(chan struct{})}
	go func() {
		_ = h.srv.run(inR)
		_ = outW.Close()
	}()
	go func() {
		defer close(h.closed)
		reader := bufio.NewReader(outR)
		for {
			raw, err := readMessage(reader)
			if err != nil {
				return
			}
			var m reply
			if err := json.Unmarshal(raw, &m); err != nil {
				t.Errorf("the helper sent something that is not JSON: %v", err)
				return
			}
			id := int64(m.num("id"))
			h.mu.Lock()
			c := h.calls[id]
			h.mu.Unlock()
			if c == nil {
				continue
			}
			if _, isEvent := m["event"]; isEvent {
				c.events <- m
			} else {
				c.done <- m
			}
		}
	}()
	t.Cleanup(h.close)
	return h
}

func (h *harness) close() {
	_ = h.toSrv.Close()
	select {
	case <-h.closed:
	case <-time.After(10 * time.Second):
		h.t.Error("the helper did not stop after its input was closed")
	}
}

func (h *harness) sendRaw(m map[string]any) {
	if err := h.out.send(m); err != nil {
		h.t.Errorf("send: %v", err)
	}
}

// start sends a request and returns the call, so the test can talk to it while it runs.
func (h *harness) start(m map[string]any) *call {
	h.mu.Lock()
	h.nextID++
	id := h.nextID
	c := &call{id: id, events: make(chan reply, 100000), done: make(chan reply, 1)}
	h.calls[id] = c
	h.mu.Unlock()
	m["id"] = id
	h.sendRaw(m)
	return c
}

func (h *harness) wait(c *call) reply {
	h.t.Helper()
	select {
	case r := <-c.done:
		return r
	case <-time.After(60 * time.Second):
		h.t.Fatalf("no answer for request %d", c.id)
		return nil
	}
}

// request sends one request and returns the answer and the events that came before it.
func (h *harness) request(m map[string]any) (reply, []reply) {
	h.t.Helper()
	c := h.start(m)
	r := h.wait(c)
	var events []reply
	for {
		select {
		case e := <-c.events:
			events = append(events, e)
		default:
			return r, events
		}
	}
}

func (h *harness) mustOK(m map[string]any) reply {
	h.t.Helper()
	r, _ := h.request(m)
	if !r.ok() {
		h.t.Fatalf("%v failed: %s: %s", m["op"], r.errCode(), r.errMessage())
	}
	return r
}

// open opens a session. `extra` overrides fields.
func (h *harness) open(session, kind, host string, port int, extra map[string]any) reply {
	h.t.Helper()
	m := map[string]any{"op": "open", "session": session, "kind": kind, "host": host, "port": port, "user": "alice", "password": "secret"}
	for k, v := range extra {
		m[k] = v
	}
	r, _ := h.request(m)
	return r
}

func (h *harness) mustOpen(session, kind, host string, port int, extra map[string]any) reply {
	h.t.Helper()
	r := h.open(session, kind, host, port, extra)
	if !r.ok() {
		h.t.Fatalf("open failed: %s: %s", r.errCode(), r.errMessage())
	}
	return r
}

type listed struct {
	Name string
	Dir  bool
	Size int64
	Time int64
}

func (h *harness) list(session, path string) ([]listed, int, reply) {
	h.t.Helper()
	r, events := h.request(map[string]any{"op": "list", "session": session, "path": path})
	var out []listed
	batches := 0
	for _, e := range events {
		if e["event"] != "entries" {
			continue
		}
		batches++
		items, _ := e["entries"].([]any)
		for _, item := range items {
			m := item.(map[string]any)
			out = append(out, listed{Name: m["n"].(string), Dir: m["d"].(bool), Size: int64(m["s"].(float64)), Time: int64(m["t"].(float64))})
		}
	}
	return out, batches, r
}

// upload sends a file the way the extension does: chunks of at most 256 KiB, no more than 6 unanswered at a time.
func (h *harness) upload(session, path string, data []byte) reply {
	h.t.Helper()
	c := h.start(map[string]any{"op": "write", "session": session, "path": path, "size": len(data)})
	select {
	case e := <-c.events:
		if e["event"] != "ready" {
			h.t.Fatalf("expected ready, got %v", e)
		}
	case r := <-c.done:
		return r
	case <-time.After(30 * time.Second):
		h.t.Fatal("the upload never got ready")
	}
	const window = 6
	sent, acked := 0, 0
	var rest = data
	for len(rest) > 0 {
		for sent-acked >= window {
			select {
			case e := <-c.events:
				if e["event"] == "ack" {
					acked += int(e.num("n"))
				}
			case r := <-c.done:
				return r
			case <-time.After(30 * time.Second):
				h.t.Fatal("no acknowledgement from the helper")
			}
		}
		n := min(256*1024, len(rest))
		h.sendRaw(map[string]any{"op": "chunk", "target": c.id, "b64": base64.StdEncoding.EncodeToString(rest[:n]), "bytes": n})
		rest = rest[n:]
		sent++
	}
	h.sendRaw(map[string]any{"op": "end", "target": c.id})
	return h.wait(c)
}

// download reads a file the way the extension does: it gives one credit for each message it has taken.
func (h *harness) download(session, path string, offset int64) ([]byte, reply, reply) {
	h.t.Helper()
	c := h.start(map[string]any{"op": "read", "session": session, "path": path, "offset": offset, "credit": 8, "chunk": 192 * 1024})
	var buf []byte
	var meta reply
	for {
		select {
		case e := <-c.events:
			switch e["event"] {
			case "meta":
				meta = e
			case "data":
				raw, err := base64.StdEncoding.DecodeString(e.str("b64"))
				if err != nil {
					h.t.Fatal(err)
				}
				buf = append(buf, raw...)
				h.sendRaw(map[string]any{"op": "credit", "target": c.id, "n": 1})
			}
		case r := <-c.done:
			// events that arrived just before the answer
			for {
				select {
				case e := <-c.events:
					if e["event"] == "data" {
						raw, _ := base64.StdEncoding.DecodeString(e.str("b64"))
						buf = append(buf, raw...)
					}
				default:
					return buf, meta, r
				}
			}
		case <-time.After(60 * time.Second):
			h.t.Fatal("the download stalled")
		}
	}
}

func sum(data []byte) string {
	s := sha256.Sum256(data)
	return hex.EncodeToString(s[:])
}

func testData(size int, seed byte) []byte {
	data := make([]byte, size)
	x := uint32(seed)*2654435761 + 1
	for i := range data {
		x = x*1664525 + 1013904223
		data[i] = byte(x >> 24)
	}
	return data
}

func names(items []listed) []string {
	out := make([]string, len(items))
	for i, item := range items {
		suffix := ""
		if item.Dir {
			suffix = "/"
		}
		out[i] = fmt.Sprintf("%s%s", item.Name, suffix)
	}
	return out
}

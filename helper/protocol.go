package main

import (
	"bufio"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"io"
	"sync"
)

// Chrome's native messaging: every message is a 4 byte length (little endian, native order on every supported
// platform) followed by that many bytes of JSON. Chrome accepts at most 1 MB from the helper and sends at most 4 GB.
const (
	maxIncoming = 32 << 20
	maxOutgoing = 1<<20 - 1024
)

// message is everything the extension can send. Fields that an operation does not use stay empty.
type message struct {
	ID      int64  `json:"id"`
	Op      string `json:"op"`
	Target  int64  `json:"target"`
	Session string `json:"session"`

	// open
	Kind     string   `json:"kind"`
	Host     string   `json:"host"`
	Port     int      `json:"port"`
	User     string   `json:"user"`
	Password string   `json:"password"`
	KeyPath  string   `json:"keyPath"`
	TLS      string   `json:"tls"`
	NoEPSV   bool     `json:"noEpsv"`
	Trusted  []string `json:"trusted"`

	// files
	Path   string `json:"path"`
	From   string `json:"from"`
	To     string `json:"to"`
	Dir    bool   `json:"dir"`
	Offset int64  `json:"offset"`
	Size   int64  `json:"size"`

	// streams
	Credit int    `json:"credit"`
	Chunk  int    `json:"chunk"`
	B64    string `json:"b64"`
	Bytes  int64  `json:"bytes"`
	N      int    `json:"n"`
}

// readMessage reads one message. It returns io.EOF when Chrome has closed the pipe.
func readMessage(r io.Reader) ([]byte, error) {
	var head [4]byte
	if _, err := io.ReadFull(r, head[:]); err != nil {
		return nil, err
	}
	size := binary.LittleEndian.Uint32(head[:])
	if size > maxIncoming {
		return nil, fmt.Errorf("message of %d bytes is too large", size)
	}
	body := make([]byte, size)
	if _, err := io.ReadFull(r, body); err != nil {
		return nil, err
	}
	return body, nil
}

// writer sends messages to Chrome. Several goroutines answer at the same time, so every message is written whole.
type writer struct {
	mu sync.Mutex
	w  *bufio.Writer
}

func newWriter(w io.Writer) *writer {
	return &writer{w: bufio.NewWriterSize(w, 64*1024)}
}

func (w *writer) send(value any) error {
	data, err := json.Marshal(value)
	if err != nil {
		return err
	}
	if len(data) > maxOutgoing {
		return fmt.Errorf("message of %d bytes is too large for Chrome", len(data))
	}
	var head [4]byte
	binary.LittleEndian.PutUint32(head[:], uint32(len(data)))
	w.mu.Lock()
	defer w.mu.Unlock()
	if _, err := w.w.Write(head[:]); err != nil {
		return err
	}
	if _, err := w.w.Write(data); err != nil {
		return err
	}
	return w.w.Flush()
}

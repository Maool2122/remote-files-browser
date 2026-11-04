package main

import (
	"bytes"
	"context"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"net"
	"net/textproto"
	"os"
	"strings"
	"syscall"
	"testing"
)

func TestFramesRoundTrip(t *testing.T) {
	var buf bytes.Buffer
	w := newWriter(&buf)
	if err := w.send(map[string]any{"id": 1, "ok": true}); err != nil {
		t.Fatal(err)
	}
	raw, err := readMessage(&buf)
	if err != nil {
		t.Fatal(err)
	}
	if string(raw) != `{"id":1,"ok":true}` {
		t.Fatalf("frame = %s", raw)
	}
	if _, err := readMessage(&buf); !errors.Is(err, io.EOF) {
		t.Fatalf("after the last frame the pipe is closed: %v", err)
	}
}

func TestFrameLengthIsLittleEndian(t *testing.T) {
	var buf bytes.Buffer
	_ = newWriter(&buf).send("abc")
	head := buf.Bytes()[:4]
	if binary.LittleEndian.Uint32(head) != 5 { // "abc" with quotes
		t.Fatalf("length bytes = %v", head)
	}
}

func TestOversizedMessagesAreRefused(t *testing.T) {
	var buf bytes.Buffer
	w := newWriter(&buf)
	if err := w.send(strings.Repeat("x", maxOutgoing+10)); err == nil {
		t.Fatal("a message larger than Chrome accepts must not be sent")
	}
	var head [4]byte
	binary.LittleEndian.PutUint32(head[:], maxIncoming+1)
	if _, err := readMessage(bytes.NewReader(head[:])); err == nil {
		t.Fatal("a huge incoming length must be refused before any memory is reserved")
	}
}

func TestTruncatedFrameIsAnError(t *testing.T) {
	var head [4]byte
	binary.LittleEndian.PutUint32(head[:], 100)
	_, err := readMessage(io.MultiReader(bytes.NewReader(head[:]), strings.NewReader("short")))
	if err == nil || errors.Is(err, io.EOF) {
		t.Fatalf("a cut frame is not a clean end: %v", err)
	}
}

func TestWireErrors(t *testing.T) {
	cases := []struct {
		name string
		err  error
		code string
		part string
	}{
		{"opError", newError("auth", "bad login"), "auth", "bad login"},
		{"cancel", fmt.Errorf("wrapped: %w", context.Canceled), "cancelled", ""},
		{"refused", &net.OpError{Op: "dial", Err: os.NewSyscallError("connect", syscall.ECONNREFUSED)}, "generic", "refused the connection"},
		{"dns", &net.DNSError{Err: "no such host", Name: "nowhere.invalid"}, "generic", "could not be found"},
		{"eof", io.ErrUnexpectedEOF, "session", ""},
		{"closed", net.ErrClosed, "session", ""},
		{"ftp421", &textproto.Error{Code: 421, Msg: "Too many connections"}, "session", ""},
		{"ftp530", &textproto.Error{Code: 530, Msg: "Login incorrect"}, "auth", ""},
		{"ftp550", &textproto.Error{Code: 550, Msg: "No such file"}, "generic", "No such file"},
		{"notexist", os.ErrNotExist, "generic", "not found"},
		{"plain", errors.New("something odd"), "generic", "something odd"},
	}
	for _, c := range cases {
		out := wire(c.err)
		if out["code"] != c.code {
			t.Errorf("%s: code = %v, want %s", c.name, out["code"], c.code)
		}
		if msg, _ := out["message"].(string); c.part != "" && !strings.Contains(msg, c.part) {
			t.Errorf("%s: message %q does not contain %q", c.name, msg, c.part)
		}
	}
}

func TestWireTrustError(t *testing.T) {
	out := wire(&trustError{Fingerprint: "ab12", Kind: "ssh-ed25519", Display: "SHA256:xyz"})
	if out["code"] != "trust" || out["fingerprint"] != "ab12" || out["fpKind"] != "ssh-ed25519" || out["display"] != "SHA256:xyz" {
		t.Fatalf("trust wire = %v", out)
	}
}

func TestTrustedSetIsCaseInsensitive(t *testing.T) {
	set := trustedSet([]string{" ABCDEF ", "012345"})
	if !set["abcdef"] || !set["012345"] || set["zzz"] {
		t.Fatalf("trusted set = %v", set)
	}
}

func TestModeText(t *testing.T) {
	if got := modeText(0o755); got != "rwxr-xr-x" {
		t.Fatalf("modeText = %q", got)
	}
}

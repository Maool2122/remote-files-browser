package main

import (
	"encoding/base64"
	"fmt"
	"sync"
	"testing"
	"time"

	ftpserver "github.com/fclairamb/ftpserverlib"
	"github.com/spf13/afero"
)

func encode(data []byte) string { return base64.StdEncoding.EncodeToString(data) }

func TestHello(t *testing.T) {
	h := newHarness(t)
	r := h.mustOK(map[string]any{"op": "hello"})
	if r.num("protocol") != protocolVersion || r.str("version") != version || r.str("os") == "" {
		t.Fatalf("hello = %v", r)
	}
}

func TestUnknownSession(t *testing.T) {
	h := newHarness(t)
	r, _ := h.request(map[string]any{"op": "list", "session": "nope", "path": "/"})
	if r.ok() || r.errCode() != "session" {
		t.Fatalf("a list on a session that is not open should say \"session\": %v", r)
	}
}

func TestFTPPlain(t *testing.T) {
	s := startFTP(t, ftpOptions{})
	h := newHarness(t)
	r := h.mustOpen("s1", "ftp", s.host, s.port, map[string]any{"tls": "none"})
	if r["secure"] != false || r.str("home") != "/" {
		t.Fatalf("open = %v", r)
	}
	exercise(t, h, "s1", "")
}

func TestFTPExplicitTLSAskForTrust(t *testing.T) {
	s := startFTP(t, ftpOptions{TLS: true})
	h := newHarness(t)

	// A certificate the computer does not know: the helper stops and shows the fingerprint.
	r := h.open("s1", "ftp", s.host, s.port, map[string]any{"tls": "explicit"})
	if r.ok() || r.errCode() != "trust" {
		t.Fatalf("expected a trust question, got %v", r)
	}
	if r.errField("fingerprint") != s.certFP || r.errField("fpKind") != "tls" || r.errField("reason") != "self-signed" {
		t.Fatalf("trust details = %v", r["error"])
	}

	// A different certificate than the one the person trusted before: the same question (the extension says "changed").
	r = h.open("s1", "ftp", s.host, s.port, map[string]any{"tls": "explicit", "trusted": []string{"00" + s.certFP[2:]}})
	if r.ok() || r.errCode() != "trust" {
		t.Fatalf("a changed certificate must be asked again: %v", r)
	}

	// Trusted: it opens, encrypted, and the whole flow works over TLS (data connections included).
	r = h.mustOpen("s1", "ftp", s.host, s.port, map[string]any{"tls": "explicit", "trusted": []string{s.certFP}})
	if r["secure"] != true {
		t.Fatalf("the link should be encrypted: %v", r)
	}
	exercise(t, h, "s1", "")
}

func TestFTPImplicitTLS(t *testing.T) {
	s := startFTP(t, ftpOptions{TLS: true, Mode: ftpserver.ImplicitEncryption})
	h := newHarness(t)
	r := h.open("s1", "ftps", s.host, s.port, map[string]any{"tls": "implicit"})
	if r.ok() || r.errCode() != "trust" || r.errField("fingerprint") != s.certFP {
		t.Fatalf("expected a trust question, got %v", r)
	}
	r = h.mustOpen("s1", "ftps", s.host, s.port, map[string]any{"tls": "implicit", "trusted": []string{s.certFP}})
	if r["secure"] != true {
		t.Fatalf("the link should be encrypted: %v", r)
	}
	exercise(t, h, "s1", "")
}

func TestFTPAutoUsesTLSWhenOffered(t *testing.T) {
	s := startFTP(t, ftpOptions{TLS: true})
	h := newHarness(t)
	r := h.mustOpen("s1", "ftp", s.host, s.port, map[string]any{"tls": "auto", "trusted": []string{s.certFP}})
	if r["secure"] != true {
		t.Fatalf("auto should use TLS when the server offers it: %v", r)
	}
}

func TestFTPAutoFallsBackWhenServerHasNoTLS(t *testing.T) {
	s := startFTP(t, ftpOptions{})
	h := newHarness(t)
	r := h.mustOpen("s1", "ftp", s.host, s.port, map[string]any{"tls": "auto"})
	if r["secure"] != false || r.str("note") != "Not encrypted" {
		t.Fatalf("auto on a server without TLS should work, not encrypted: %v", r)
	}
}

func TestFTPWrongPasswordIsAuthAndNeverDowngrades(t *testing.T) {
	s := startFTP(t, ftpOptions{TLS: true})
	h := newHarness(t)
	r := h.open("s1", "ftp", s.host, s.port, map[string]any{"tls": "auto", "trusted": []string{s.certFP}, "password": "wrong"})
	if r.ok() || r.errCode() != "auth" {
		t.Fatalf("a wrong password should be \"auth\": %v", r)
	}
	// An explicit "none" on a TLS-only server is refused, not silently upgraded.
	s2 := startFTP(t, ftpOptions{TLS: true, Mode: ftpserver.MandatoryEncryption})
	r = h.open("s2", "ftp", s2.host, s2.port, map[string]any{"tls": "none"})
	if r.ok() {
		t.Fatalf("a server that requires TLS must not be reachable with tls=none: %v", r)
	}
}

func TestFTPAnonymous(t *testing.T) {
	s := startFTP(t, ftpOptions{})
	_ = afero.WriteFile(s.fs, "/pub/readme.txt", []byte("public"), 0o644)
	h := newHarness(t)
	h.mustOpen("s1", "ftp", s.host, s.port, map[string]any{"tls": "none", "user": "", "password": ""})
	items, _, r := h.list("s1", "/pub")
	if !r.ok() || len(items) != 1 || items[0].Name != "readme.txt" {
		t.Fatalf("anonymous list = %v %v", names(items), r)
	}
}

func TestFTPBigListingIsSentInBatches(t *testing.T) {
	s := startFTP(t, ftpOptions{})
	for i := 0; i < 3000; i++ {
		_ = afero.WriteFile(s.fs, fmt.Sprintf("/many/file-%05d.txt", i), []byte("x"), 0o644)
	}
	h := newHarness(t)
	h.mustOpen("s1", "ftp", s.host, s.port, map[string]any{"tls": "none"})
	items, batches, r := h.list("s1", "/many")
	if !r.ok() || len(items) != 3000 {
		t.Fatalf("listing: %d entries, %v", len(items), r)
	}
	if batches < 6 {
		t.Errorf("3000 entries should come in several messages, got %d", batches)
	}
}

func TestFTPCancelDownloadKeepsSessionUsable(t *testing.T) {
	s := startFTP(t, ftpOptions{})
	_ = afero.WriteFile(s.fs, "/big/huge.bin", testData(30_000_000, 1), 0o644)
	h := newHarness(t)
	h.mustOpen("s1", "ftp", s.host, s.port, map[string]any{"tls": "none"})

	// Give only the 8 first credits and never more: the helper must wait, not pile data up.
	c := h.start(map[string]any{"op": "read", "session": "s1", "path": "/big/huge.bin", "offset": 0, "credit": 8})
	dataEvents := 0
	deadline := time.After(20 * time.Second)
	for dataEvents < 8 {
		select {
		case e := <-c.events:
			if e["event"] == "data" {
				dataEvents++
			}
		case <-deadline:
			t.Fatal("the download did not start")
		}
	}
	time.Sleep(700 * time.Millisecond)
	extra := 0
	for len(c.events) > 0 {
		if e := <-c.events; e["event"] == "data" {
			extra++
		}
	}
	if extra != 0 {
		t.Errorf("the helper sent %d messages beyond the credit it was given", extra)
	}
	h.sendRaw(map[string]any{"op": "cancel", "target": c.id})
	r := h.wait(c)
	if r.ok() || r.errCode() != "cancelled" {
		t.Fatalf("cancel answer = %v", r)
	}
	items, _, lr := h.list("s1", "/big")
	if !lr.ok() || len(items) != 1 {
		t.Fatalf("list after cancel: %v %v", names(items), lr)
	}
}

func TestFTPParallelDownloads(t *testing.T) {
	s := startFTP(t, ftpOptions{})
	const files = 12
	want := map[string]string{}
	for i := 0; i < files; i++ {
		data := testData(1_500_000+i, byte(i))
		_ = afero.WriteFile(s.fs, fmt.Sprintf("/par/f%d.bin", i), data, 0o644)
		want[fmt.Sprintf("/par/f%d.bin", i)] = sum(data)
	}
	h := newHarness(t)
	h.mustOpen("s1", "ftp", s.host, s.port, map[string]any{"tls": "none"})
	var wg sync.WaitGroup
	for path, hash := range want {
		wg.Add(1)
		go func() {
			defer wg.Done()
			data, _, r := h.download("s1", path, 0)
			if !r.ok() || sum(data) != hash {
				t.Errorf("parallel download of %s failed: ok=%v len=%d %s", path, r.ok(), len(data), r.errMessage())
			}
		}()
	}
	wg.Wait()
	if peak := s.peakConnections(); peak > ftpMaxConnections || peak < 2 {
		t.Errorf("peak connections = %d, want between 2 and %d", peak, ftpMaxConnections)
	}
}

func TestFTPServerWithConnectionLimit(t *testing.T) {
	s := startFTP(t, ftpOptions{MaxConns: 2})
	const files = 8
	want := map[string]string{}
	for i := 0; i < files; i++ {
		data := testData(800_000+i, byte(i))
		_ = afero.WriteFile(s.fs, fmt.Sprintf("/lim/f%d.bin", i), data, 0o644)
		want[fmt.Sprintf("/lim/f%d.bin", i)] = sum(data)
	}
	h := newHarness(t)
	h.mustOpen("s1", "ftp", s.host, s.port, map[string]any{"tls": "none"})
	var wg sync.WaitGroup
	for path, hash := range want {
		wg.Add(1)
		go func() {
			defer wg.Done()
			data, _, r := h.download("s1", path, 0)
			if !r.ok() || sum(data) != hash {
				t.Errorf("download of %s with a server limit failed: ok=%v len=%d %s %s", path, r.ok(), len(data), r.errCode(), r.errMessage())
			}
		}()
	}
	wg.Wait()
	if peak := s.peakConnections(); peak > 2 {
		t.Errorf("the server allows 2 connections, the helper used %d", peak)
	}
}

func TestFTPCloseThenUse(t *testing.T) {
	s := startFTP(t, ftpOptions{})
	h := newHarness(t)
	h.mustOpen("s1", "ftp", s.host, s.port, map[string]any{"tls": "none"})
	h.mustOK(map[string]any{"op": "close", "session": "s1"})
	r, _ := h.request(map[string]any{"op": "list", "session": "s1", "path": "/"})
	if r.ok() || r.errCode() != "session" {
		t.Fatalf("after close the session is gone: %v", r)
	}
}

func TestFTPUnreachableServer(t *testing.T) {
	h := newHarness(t)
	r := h.open("s1", "ftp", "127.0.0.1", 1, map[string]any{"tls": "none"})
	if r.ok() || r.errMessage() == "" {
		t.Fatalf("an unreachable server must give an error with words: %v", r)
	}
}

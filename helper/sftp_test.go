package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
)

func TestSFTPPasswordFlow(t *testing.T) {
	s := startSFTP(t)
	h := newHarness(t)

	// First contact: the host key is new, so the helper stops and shows it.
	r := h.open("s1", "sftp", s.host, s.port, nil)
	if r.ok() || r.errCode() != "trust" {
		t.Fatalf("expected a trust question, got %v", r)
	}
	if r.errField("fingerprint") != s.hostFP || r.errField("fpKind") != "ssh-ed25519" || r.errField("display") != s.hostDisplay {
		t.Fatalf("trust details = %v (want %s / %s)", r["error"], s.hostFP, s.hostDisplay)
	}

	// A different key than the trusted one: the same question again.
	r = h.open("s1", "sftp", s.host, s.port, map[string]any{"trusted": []string{"00" + s.hostFP[2:]}})
	if r.ok() || r.errCode() != "trust" {
		t.Fatalf("a changed host key must be asked again: %v", r)
	}

	r = h.mustOpen("s1", "sftp", s.host, s.port, map[string]any{"trusted": []string{s.hostFP}})
	if r["secure"] != true || r.str("note") != "Encrypted (SSH)" {
		t.Fatalf("open = %v", r)
	}
	exercise(t, h, "s1", "")
}

func TestSFTPWrongPassword(t *testing.T) {
	s := startSFTP(t)
	h := newHarness(t)
	r := h.open("s1", "sftp", s.host, s.port, map[string]any{"trusted": []string{s.hostFP}, "password": "nope"})
	if r.ok() || r.errCode() != "auth" {
		t.Fatalf("a wrong password should be \"auth\": %v", r)
	}
	r = h.open("s1", "sftp", s.host, s.port, map[string]any{"trusted": []string{s.hostFP}, "user": "", "password": "secret"})
	if r.ok() || r.errCode() != "auth" {
		t.Fatalf("a missing user name should be \"auth\": %v", r)
	}
}

func TestSFTPKeyFiles(t *testing.T) {
	s := startSFTP(t)
	h := newHarness(t)
	dir := t.TempDir()
	plain := filepath.Join(dir, "id_plain")
	protected := filepath.Join(dir, "id_protected")
	if err := os.WriteFile(plain, s.userKeyPEM, 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(protected, s.protectedPEM, 0o600); err != nil {
		t.Fatal(err)
	}
	trusted := []string{s.hostFP}

	h.mustOpen("k1", "sftp", s.host, s.port, map[string]any{"trusted": trusted, "keyPath": plain, "password": ""})
	h.mustOpen("k2", "sftp", s.host, s.port, map[string]any{"trusted": trusted, "keyPath": protected, "password": "phrase"})

	r := h.open("k3", "sftp", s.host, s.port, map[string]any{"trusted": trusted, "keyPath": protected, "password": ""})
	if r.ok() || r.errCode() != "auth" || !strings.Contains(r.errMessage(), "passphrase") {
		t.Fatalf("a protected key without its passphrase: %v", r)
	}
	r = h.open("k4", "sftp", s.host, s.port, map[string]any{"trusted": trusted, "keyPath": protected, "password": "wrong"})
	if r.ok() || r.errCode() != "auth" {
		t.Fatalf("a protected key with a wrong passphrase: %v", r)
	}
	r = h.open("k5", "sftp", s.host, s.port, map[string]any{"trusted": trusted, "keyPath": filepath.Join(dir, "missing")})
	if r.ok() || r.errMessage() == "" {
		t.Fatalf("a missing key file must give a message: %v", r)
	}
	notAKey := filepath.Join(dir, "notes.txt")
	_ = os.WriteFile(notAKey, []byte("this is not a key"), 0o600)
	r = h.open("k6", "sftp", s.host, s.port, map[string]any{"trusted": trusted, "keyPath": notAKey})
	if r.ok() || r.errMessage() == "" {
		t.Fatalf("a file that is not a key must give a message: %v", r)
	}
}

func TestSFTPParallelTransfers(t *testing.T) {
	s := startSFTP(t)
	h := newHarness(t)
	h.mustOpen("s1", "sftp", s.host, s.port, map[string]any{"trusted": []string{s.hostFP}})

	const files = 8
	var wg sync.WaitGroup
	for i := 0; i < files; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			data := testData(1_200_000+i*17, byte(i))
			path := fmt.Sprintf("/par-%d.bin", i)
			if r := h.upload("s1", path, data); !r.ok() {
				t.Errorf("upload %s: %s", path, r.errMessage())
				return
			}
			got, _, r := h.download("s1", path, 0)
			if !r.ok() || sum(got) != sum(data) {
				t.Errorf("round trip of %s failed: ok=%v len=%d want=%d", path, r.ok(), len(got), len(data))
			}
		}()
	}
	wg.Wait()
}

func TestSFTPUnreachableServer(t *testing.T) {
	h := newHarness(t)
	r := h.open("s1", "sftp", "127.0.0.1", 1, nil)
	if r.ok() || r.errMessage() == "" {
		t.Fatalf("an unreachable server must give an error with words: %v", r)
	}
}

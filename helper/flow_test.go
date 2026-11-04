package main

import (
	"bytes"
	"reflect"
	"sort"
	"testing"
	"time"
)

// exercise runs the same set of file operations on any open session. Every protocol must pass it.
func exercise(t *testing.T, h *harness, session, root string) {
	t.Helper()
	nested := root + "/work/deep/er"
	spaced := root + "/work/with space"

	// Folders: nested ones in one go, and doing it twice is fine.
	h.mustOK(map[string]any{"op": "mkdir", "session": session, "path": nested})
	h.mustOK(map[string]any{"op": "mkdir", "session": session, "path": nested})
	h.mustOK(map[string]any{"op": "mkdir", "session": session, "path": spaced})

	small := []byte("hello, world")
	big := testData(2_500_000, 7)
	unicode := []byte("ünï-çödé")
	for path, data := range map[string][]byte{
		root + "/work/small.txt":    small,
		root + "/work/deep/big.bin": big,
		root + "/work/empty.txt":    {},
		spaced + "/ünï file.txt":    unicode,
		nested + "/in-the-deep.txt": []byte("x"),
	} {
		if r := h.upload(session, path, data); !r.ok() {
			t.Fatalf("upload %s: %s: %s", path, r.errCode(), r.errMessage())
		}
	}

	// Listing
	items, _, r := h.list(session, root+"/work")
	if !r.ok() {
		t.Fatalf("list: %s: %s", r.errCode(), r.errMessage())
	}
	got := names(items)
	sort.Strings(got)
	want := []string{"deep/", "empty.txt", "small.txt", "with space/"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("list = %v, want %v", got, want)
	}
	for _, item := range items {
		if item.Name == "small.txt" && item.Size != int64(len(small)) {
			t.Errorf("small.txt size = %d", item.Size)
		}
		if item.Name == "empty.txt" && item.Size != 0 {
			t.Errorf("empty.txt size = %d", item.Size)
		}
	}
	inner, _, _ := h.list(session, spaced)
	if len(inner) != 1 || inner[0].Name != "ünï file.txt" {
		t.Fatalf("list of the folder with a space = %v", names(inner))
	}

	// Stat
	st := h.mustOK(map[string]any{"op": "stat", "session": session, "path": root + "/work/small.txt"})
	if st["exists"] != true || st["dir"] != false || st.num("size") != int64(len(small)) {
		t.Errorf("stat of a file = %v", st)
	}
	st = h.mustOK(map[string]any{"op": "stat", "session": session, "path": root + "/work/deep"})
	if st["exists"] != true || st["dir"] != true {
		t.Errorf("stat of a folder = %v", st)
	}
	st = h.mustOK(map[string]any{"op": "stat", "session": session, "path": root + "/work/missing.txt"})
	if st["exists"] != false {
		t.Errorf("stat of a missing file = %v", st)
	}

	// Download, whole and from the middle
	data, meta, r := h.download(session, root+"/work/small.txt", 0)
	if !r.ok() || !bytes.Equal(data, small) || meta.num("size") != int64(len(small)) {
		t.Fatalf("download small: %v %q meta=%v", r, data, meta)
	}
	data, _, r = h.download(session, root+"/work/empty.txt", 0)
	if !r.ok() || len(data) != 0 {
		t.Fatalf("download empty: %v len=%d", r, len(data))
	}
	data, meta, r = h.download(session, root+"/work/deep/big.bin", 0)
	if !r.ok() || sum(data) != sum(big) {
		t.Fatalf("download big: ok=%v len=%d want=%d (%s)", r.ok(), len(data), len(big), r.errMessage())
	}
	if meta.num("size") != int64(len(big)) {
		t.Errorf("big size in meta = %d", meta.num("size"))
	}
	const offset = 1_000_003
	data, meta, r = h.download(session, root+"/work/deep/big.bin", offset)
	if !r.ok() || sum(data) != sum(big[offset:]) {
		t.Fatalf("resume: ok=%v len=%d want=%d (%s)", r.ok(), len(data), len(big)-offset, r.errMessage())
	}
	if meta["resumed"] != true {
		t.Errorf("the server should have resumed: %v", meta)
	}
	data, _, r = h.download(session, spaced+"/ünï file.txt", 0)
	if !r.ok() || !bytes.Equal(data, unicode) {
		t.Fatalf("download with accents: %v %q", r, data)
	}

	// A file that is not there is an error with words, not a crash.
	_, _, r = h.download(session, root+"/work/missing.txt", 0)
	if r.ok() || r.errMessage() == "" {
		t.Fatalf("a missing file should fail with a message, got %v", r)
	}

	// Rename
	h.mustOK(map[string]any{"op": "rename", "session": session, "from": root + "/work/small.txt", "to": root + "/work/renamed.txt"})
	st = h.mustOK(map[string]any{"op": "stat", "session": session, "path": root + "/work/renamed.txt"})
	if st["exists"] != true {
		t.Errorf("renamed file is missing: %v", st)
	}
	st = h.mustOK(map[string]any{"op": "stat", "session": session, "path": root + "/work/small.txt"})
	if st["exists"] != false {
		t.Errorf("old name still there: %v", st)
	}

	// Remove a file, then a whole folder with things inside
	h.mustOK(map[string]any{"op": "remove", "session": session, "path": root + "/work/renamed.txt", "dir": false})
	h.mustOK(map[string]any{"op": "remove", "session": session, "path": root + "/work/deep", "dir": true})
	st = h.mustOK(map[string]any{"op": "stat", "session": session, "path": root + "/work/deep"})
	if st["exists"] != false {
		t.Errorf("folder still there after removal: %v", st)
	}
	items, _, _ = h.list(session, root+"/work")
	got = names(items)
	sort.Strings(got)
	want = []string{"empty.txt", "with space/"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("list after removal = %v, want %v", got, want)
	}

	// Upload that is cancelled half way leaves nothing behind.
	cancelled := root + "/work/cancelled.bin"
	c := h.start(map[string]any{"op": "write", "session": session, "path": cancelled, "size": 5_000_000})
	select {
	case e := <-c.events:
		if e["event"] != "ready" {
			t.Fatalf("expected ready: %v", e)
		}
	case <-time.After(30 * time.Second):
		t.Fatal("not ready")
	}
	piece := testData(256*1024, 3)
	for i := 0; i < 3; i++ {
		h.sendRaw(map[string]any{"op": "chunk", "target": c.id, "b64": encode(piece), "bytes": len(piece)})
	}
	h.sendRaw(map[string]any{"op": "cancel", "target": c.id})
	r = h.wait(c)
	if r.ok() || r.errCode() != "cancelled" {
		t.Fatalf("cancelled upload answer = %v", r)
	}
	// Give a server that deletes in the background a moment.
	deadline := time.Now().Add(10 * time.Second)
	for {
		st = h.mustOK(map[string]any{"op": "stat", "session": session, "path": cancelled})
		if st["exists"] == false {
			break
		}
		if time.Now().After(deadline) {
			t.Fatalf("the half uploaded file was left on the server")
		}
		time.Sleep(200 * time.Millisecond)
	}

	// The session still works after the cancel.
	if r := h.upload(session, root+"/work/after.txt", []byte("still works")); !r.ok() {
		t.Fatalf("upload after cancel: %s: %s", r.errCode(), r.errMessage())
	}
}

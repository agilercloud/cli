package fsx

import (
	"io"
	"testing"
	"time"
)

func TestMemFSRoundtrip(t *testing.T) {
	fs := NewMemFS()
	now := time.Date(2025, 1, 2, 3, 4, 5, 0, time.UTC)
	fs.WriteFile("/a/b/c.txt", []byte("hello"), now)

	info, err := fs.Stat("/a/b/c.txt")
	if err != nil {
		t.Fatalf("Stat: %v", err)
	}
	if info.Size() != 5 {
		t.Errorf("Size = %d, want 5", info.Size())
	}
	if !info.ModTime().Equal(now) {
		t.Errorf("ModTime = %v, want %v", info.ModTime(), now)
	}

	rc, err := fs.Open("/a/b/c.txt")
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	b, _ := io.ReadAll(rc)
	rc.Close()
	if string(b) != "hello" {
		t.Errorf("Read = %q, want %q", b, "hello")
	}
}

func TestMemFSReadDir(t *testing.T) {
	fs := NewMemFS()
	fs.WriteFile("/top/a.txt", []byte("a"), time.Now())
	fs.WriteFile("/top/b.txt", []byte("b"), time.Now())
	fs.WriteFile("/top/sub/c.txt", []byte("c"), time.Now())

	entries, err := fs.ReadDir("/top")
	if err != nil {
		t.Fatalf("ReadDir: %v", err)
	}
	names := []string{}
	for _, e := range entries {
		names = append(names, e.Name())
	}
	wantSet := map[string]bool{"a.txt": true, "b.txt": true, "sub": true}
	for _, n := range names {
		if !wantSet[n] {
			t.Errorf("unexpected entry %q", n)
		}
		delete(wantSet, n)
	}
	if len(wantSet) > 0 {
		t.Errorf("missing entries: %v", wantSet)
	}
}

func TestMemFSCreate(t *testing.T) {
	fs := NewMemFS()
	w, err := fs.Create("/x/y.txt")
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	if _, err := w.Write([]byte("data")); err != nil {
		t.Fatalf("Write: %v", err)
	}
	w.Close()

	info, err := fs.Stat("/x/y.txt")
	if err != nil {
		t.Fatalf("Stat: %v", err)
	}
	if info.Size() != 4 {
		t.Errorf("Size = %d, want 4", info.Size())
	}
}

func TestMemFSChtimes(t *testing.T) {
	fs := NewMemFS()
	fs.WriteFile("/t.txt", []byte("x"), time.Now())

	want := time.Date(2020, 6, 1, 12, 0, 0, 0, time.UTC)
	if err := fs.Chtimes("/t.txt", want, want); err != nil {
		t.Fatalf("Chtimes: %v", err)
	}
	info, _ := fs.Stat("/t.txt")
	if !info.ModTime().Equal(want) {
		t.Errorf("ModTime = %v, want %v", info.ModTime(), want)
	}
}

func TestMemFSStatMissing(t *testing.T) {
	fs := NewMemFS()
	if _, err := fs.Stat("/missing"); err == nil {
		t.Error("expected error for missing file")
	}
}

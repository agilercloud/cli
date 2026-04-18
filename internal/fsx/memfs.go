package fsx

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"
)

// MemFS is an in-memory FS implementation for tests.
// Paths are normalized with filepath.Clean; directories are tracked
// explicitly so ReadDir returns them.
type MemFS struct {
	mu    sync.RWMutex
	files map[string]*memEntry
}

type memEntry struct {
	data    []byte
	mode    os.FileMode
	modTime time.Time
	isDir   bool
}

// NewMemFS returns an empty MemFS.
func NewMemFS() *MemFS {
	return &MemFS{files: map[string]*memEntry{}}
}

func cleanPath(name string) string {
	return filepath.Clean(name)
}

// WriteFile is a test helper to seed a file into the FS.
func (m *MemFS) WriteFile(name string, data []byte, modTime time.Time) {
	m.mu.Lock()
	defer m.mu.Unlock()
	name = cleanPath(name)
	m.mkdirAllLocked(filepath.Dir(name))
	m.files[name] = &memEntry{
		data:    append([]byte(nil), data...),
		mode:    0o644,
		modTime: modTime,
	}
}

func (m *MemFS) mkdirAllLocked(dir string) {
	if dir == "." || dir == "/" || dir == "" {
		return
	}
	if _, ok := m.files[dir]; ok {
		return
	}
	m.mkdirAllLocked(filepath.Dir(dir))
	m.files[dir] = &memEntry{
		mode:    0o755 | os.ModeDir,
		modTime: time.Now(),
		isDir:   true,
	}
}

func (m *MemFS) MkdirAll(name string, perm os.FileMode) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.mkdirAllLocked(cleanPath(name))
	return nil
}

func (m *MemFS) Stat(name string) (os.FileInfo, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	e, ok := m.files[cleanPath(name)]
	if !ok {
		return nil, &os.PathError{Op: "stat", Path: name, Err: os.ErrNotExist}
	}
	return &memFileInfo{name: filepath.Base(name), entry: e}, nil
}

func (m *MemFS) Open(name string) (io.ReadCloser, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	e, ok := m.files[cleanPath(name)]
	if !ok {
		return nil, &os.PathError{Op: "open", Path: name, Err: os.ErrNotExist}
	}
	if e.isDir {
		return nil, &os.PathError{Op: "open", Path: name, Err: fmt.Errorf("is a directory")}
	}
	return io.NopCloser(bytes.NewReader(e.data)), nil
}

func (m *MemFS) Create(name string) (io.WriteCloser, error) {
	name = cleanPath(name)
	m.mu.Lock()
	m.mkdirAllLocked(filepath.Dir(name))
	e := &memEntry{mode: 0o644, modTime: time.Now()}
	m.files[name] = e
	m.mu.Unlock()
	return &memWriter{fs: m, name: name, entry: e}, nil
}

func (m *MemFS) ReadDir(name string) ([]os.DirEntry, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	name = cleanPath(name)
	if e, ok := m.files[name]; ok && !e.isDir {
		return nil, &os.PathError{Op: "readdir", Path: name, Err: fmt.Errorf("not a directory")}
	}
	var entries []os.DirEntry
	prefix := name
	if !strings.HasSuffix(prefix, string(filepath.Separator)) {
		prefix += string(filepath.Separator)
	}
	for p, e := range m.files {
		if p == name {
			continue
		}
		if !strings.HasPrefix(p, prefix) {
			continue
		}
		rest := strings.TrimPrefix(p, prefix)
		if strings.Contains(rest, string(filepath.Separator)) {
			continue
		}
		entries = append(entries, &memDirEntry{name: path.Base(p), entry: e})
	}
	sort.Slice(entries, func(i, j int) bool { return entries[i].Name() < entries[j].Name() })
	return entries, nil
}

func (m *MemFS) Chtimes(name string, atime, mtime time.Time) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	e, ok := m.files[cleanPath(name)]
	if !ok {
		return &os.PathError{Op: "chtimes", Path: name, Err: os.ErrNotExist}
	}
	e.modTime = mtime
	return nil
}

type memWriter struct {
	fs    *MemFS
	name  string
	entry *memEntry
	buf   bytes.Buffer
}

func (w *memWriter) Write(p []byte) (int, error) { return w.buf.Write(p) }
func (w *memWriter) Close() error {
	w.fs.mu.Lock()
	defer w.fs.mu.Unlock()
	w.entry.data = append([]byte(nil), w.buf.Bytes()...)
	return nil
}

type memFileInfo struct {
	name  string
	entry *memEntry
}

func (i *memFileInfo) Name() string       { return i.name }
func (i *memFileInfo) Size() int64        { return int64(len(i.entry.data)) }
func (i *memFileInfo) Mode() os.FileMode  { return i.entry.mode }
func (i *memFileInfo) ModTime() time.Time { return i.entry.modTime }
func (i *memFileInfo) IsDir() bool        { return i.entry.isDir }
func (i *memFileInfo) Sys() any           { return nil }

type memDirEntry struct {
	name  string
	entry *memEntry
}

func (d *memDirEntry) Name() string { return d.name }
func (d *memDirEntry) IsDir() bool  { return d.entry.isDir }
func (d *memDirEntry) Type() os.FileMode {
	if d.entry.isDir {
		return os.ModeDir
	}
	return 0
}
func (d *memDirEntry) Info() (os.FileInfo, error) {
	return &memFileInfo{name: d.name, entry: d.entry}, nil
}

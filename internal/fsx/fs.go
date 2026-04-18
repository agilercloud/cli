// Package fsx is a minimal write-capable filesystem abstraction the CLI
// uses so sync code can be tested against an in-memory implementation.
//
// io/fs.FS is read-only; fsx adds Create, MkdirAll, and Chtimes. The
// interface deliberately mirrors the subset of os/* functions the CLI
// actually uses — resist expanding it.
package fsx

import (
	"io"
	"os"
	"time"
)

// FS is implemented by both OSFS (real filesystem) and MemFS (tests).
type FS interface {
	ReadDir(name string) ([]os.DirEntry, error)
	Open(name string) (io.ReadCloser, error)
	Create(name string) (io.WriteCloser, error)
	Stat(name string) (os.FileInfo, error)
	MkdirAll(name string, perm os.FileMode) error
	Chtimes(name string, atime, mtime time.Time) error
}

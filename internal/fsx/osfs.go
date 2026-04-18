package fsx

import (
	"io"
	"os"
	"time"
)

// OSFS is the FS implementation that delegates to the os package.
type OSFS struct{}

func (OSFS) ReadDir(name string) ([]os.DirEntry, error) { return os.ReadDir(name) }
func (OSFS) Open(name string) (io.ReadCloser, error)    { return os.Open(name) }
func (OSFS) Create(name string) (io.WriteCloser, error) { return os.Create(name) }
func (OSFS) Stat(name string) (os.FileInfo, error)      { return os.Stat(name) }
func (OSFS) MkdirAll(name string, perm os.FileMode) error {
	return os.MkdirAll(name, perm)
}
func (OSFS) Chtimes(name string, atime, mtime time.Time) error {
	return os.Chtimes(name, atime, mtime)
}

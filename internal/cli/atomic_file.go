package cli

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
)

// writeStreamAtomic copies src into a sibling temp file of outputPath and
// renames it on success. A failed copy leaves any existing destination
// untouched. The caller decides whether the destination's parent directory
// should be created before invoking this helper.
func writeStreamAtomic(outputPath string, src io.Reader) (int64, error) {
	tmp, err := os.CreateTemp(filepath.Dir(outputPath), ".agiler-download-*")
	if err != nil {
		return 0, fmt.Errorf("create temp file: %w", err)
	}
	tmpPath := tmp.Name()
	cleanup := true
	defer func() {
		_ = tmp.Close()
		if cleanup {
			_ = os.Remove(tmpPath)
		}
	}()

	n, err := io.Copy(tmp, src)
	if err != nil {
		return n, fmt.Errorf("write file: %w", err)
	}
	if err := tmp.Close(); err != nil {
		return n, fmt.Errorf("close temp: %w", err)
	}
	if err := os.Rename(tmpPath, outputPath); err != nil {
		return n, fmt.Errorf("finalize: %w", err)
	}
	cleanup = false
	return n, nil
}

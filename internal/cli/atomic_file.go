package cli

import (
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"

	"github.com/agilercloud/cli/internal/app"
	"github.com/agilercloud/cli/internal/output"
)

type downloadResponseOptions struct {
	OutputPath   string
	ShowProgress bool
	BeforeWrite  func() error
	AfterWrite   func()
}

// writeDownloadResponse writes a streaming HTTP response to stdout or an
// atomic destination file and always closes the response body. File-specific
// setup and metadata preservation remain optional caller hooks.
func writeDownloadResponse(a *app.App, resp *http.Response, opts downloadResponseOptions) error {
	defer func() { _ = resp.Body.Close() }()

	if opts.OutputPath == "" || opts.OutputPath == "-" {
		if _, err := io.Copy(a.Out, resp.Body); err != nil {
			return fmt.Errorf("write file: %w", err)
		}
		return nil
	}

	if opts.BeforeWrite != nil {
		if err := opts.BeforeWrite(); err != nil {
			return err
		}
	}

	body := io.Reader(resp.Body)
	var prog *output.ProgressReader
	if opts.ShowProgress && a.Output.ErrColor.Enabled() {
		prog = output.NewProgressReader(
			resp.Body,
			a.Err,
			filepath.Base(opts.OutputPath),
			resp.ContentLength,
			a.Output.ErrColor,
		)
		body = prog
	}

	n, err := writeStreamAtomic(opts.OutputPath, body)
	if prog != nil {
		prog.Finish(err == nil)
	}
	if err != nil {
		return err
	}

	if opts.AfterWrite != nil {
		opts.AfterWrite()
	}
	if prog == nil {
		a.Output.Stderr("Downloaded %d bytes to %s", n, opts.OutputPath)
	}
	return nil
}

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

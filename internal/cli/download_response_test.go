package cli

import (
	"errors"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

type trackingReadCloser struct {
	io.Reader
	closed bool
}

func (r *trackingReadCloser) Close() error {
	r.closed = true
	return nil
}

type eventReader struct {
	reader io.Reader
	events *[]string
	read   bool
}

func (r *eventReader) Read(p []byte) (int, error) {
	if !r.read {
		r.read = true
		*r.events = append(*r.events, "read")
	}
	return r.reader.Read(p)
}

func TestWriteDownloadResponseStreamsStdoutAndClosesBody(t *testing.T) {
	for _, tc := range []struct {
		name       string
		outputPath string
	}{
		{name: "empty output", outputPath: ""},
		{name: "dash output", outputPath: "-"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			a, out, errOut := newTestApp(t)
			body := &trackingReadCloser{Reader: strings.NewReader("payload")}
			beforeCalled := false
			afterCalled := false

			err := writeDownloadResponse(a, &http.Response{
				Body:          body,
				ContentLength: int64(len("payload")),
			}, downloadResponseOptions{
				OutputPath:   tc.outputPath,
				ShowProgress: true,
				BeforeWrite: func() error {
					beforeCalled = true
					return nil
				},
				AfterWrite: func() {
					afterCalled = true
				},
			})
			if err != nil {
				t.Fatalf("writeDownloadResponse: %v", err)
			}
			if got := out.String(); got != "payload" {
				t.Fatalf("stdout = %q, want payload", got)
			}
			if got := errOut.String(); got != "" {
				t.Fatalf("stderr = %q, want empty", got)
			}
			if beforeCalled || afterCalled {
				t.Fatalf("file hooks called for stdout: before=%v after=%v", beforeCalled, afterCalled)
			}
			if !body.closed {
				t.Fatal("response body was not closed")
			}
		})
	}
}

func TestWriteDownloadResponseWritesAtomicallyRunsHooksAndReportsBytes(t *testing.T) {
	a, out, errOut := newTestApp(t)
	destination := filepath.Join(t.TempDir(), "nested", "artifact.bin")
	events := []string{}
	reader := &eventReader{
		reader: strings.NewReader("complete"),
		events: &events,
	}
	body := &trackingReadCloser{Reader: reader}

	err := writeDownloadResponse(a, &http.Response{
		Body:          body,
		ContentLength: int64(len("complete")),
	}, downloadResponseOptions{
		OutputPath:   destination,
		ShowProgress: true,
		BeforeWrite: func() error {
			events = append(events, "before")
			return os.MkdirAll(filepath.Dir(destination), 0o755)
		},
		AfterWrite: func() {
			events = append(events, "after")
		},
	})
	if err != nil {
		t.Fatalf("writeDownloadResponse: %v", err)
	}

	got, err := os.ReadFile(destination)
	if err != nil {
		t.Fatalf("read destination: %v", err)
	}
	if string(got) != "complete" {
		t.Fatalf("destination = %q, want complete", got)
	}
	if !slices.Equal(events, []string{"before", "read", "after"}) {
		t.Fatalf("events = %v, want [before read after]", events)
	}
	if got := out.String(); got != "" {
		t.Fatalf("stdout = %q, want empty", got)
	}
	wantReport := "Downloaded 8 bytes to " + destination + "\n"
	if got := errOut.String(); got != wantReport {
		t.Fatalf("stderr = %q, want %q", got, wantReport)
	}
	if !body.closed {
		t.Fatal("response body was not closed")
	}
}

func TestWriteDownloadResponseSkipsAfterHookOnStreamFailure(t *testing.T) {
	a, _, errOut := newTestApp(t)
	destination := filepath.Join(t.TempDir(), "artifact.bin")
	if err := os.WriteFile(destination, []byte("original"), 0o644); err != nil {
		t.Fatal(err)
	}
	body := &trackingReadCloser{Reader: &failingReader{
		data: []byte("partial"),
		err:  errors.New("connection reset"),
	}}
	afterCalled := false

	err := writeDownloadResponse(a, &http.Response{Body: body}, downloadResponseOptions{
		OutputPath: destination,
		AfterWrite: func() {
			afterCalled = true
		},
	})
	if err == nil {
		t.Fatal("expected stream failure")
	}
	if afterCalled {
		t.Fatal("after hook called after failed write")
	}
	if !body.closed {
		t.Fatal("response body was not closed")
	}
	if got := errOut.String(); got != "" {
		t.Fatalf("stderr = %q, want no completion report", got)
	}
	got, readErr := os.ReadFile(destination)
	if readErr != nil {
		t.Fatal(readErr)
	}
	if string(got) != "original" {
		t.Fatalf("destination = %q, want original", got)
	}
}

func TestWriteDownloadResponseClosesBodyWhenBeforeHookFails(t *testing.T) {
	a, _, _ := newTestApp(t)
	body := &trackingReadCloser{Reader: strings.NewReader("payload")}
	hookErr := errors.New("prepare destination")
	afterCalled := false

	err := writeDownloadResponse(a, &http.Response{Body: body}, downloadResponseOptions{
		OutputPath: filepath.Join(t.TempDir(), "artifact.bin"),
		BeforeWrite: func() error {
			return hookErr
		},
		AfterWrite: func() {
			afterCalled = true
		},
	})
	if !errors.Is(err, hookErr) {
		t.Fatalf("error = %v, want %v", err, hookErr)
	}
	if afterCalled {
		t.Fatal("after hook called after before hook failed")
	}
	if !body.closed {
		t.Fatal("response body was not closed")
	}
}

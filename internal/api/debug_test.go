package api

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func newDebugClient(t *testing.T, h http.HandlerFunc, w io.Writer) (*http.Client, string, func()) {
	t.Helper()
	srv := httptest.NewServer(h)
	transport := newDebugTransport(http.DefaultTransport, w)
	return &http.Client{Transport: transport}, srv.URL, srv.Close
}

func TestDebugTransportLogsMethodURLAndStatus(t *testing.T) {
	var buf bytes.Buffer
	client, base, cleanup := newDebugClient(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"ok":true}`))
	}, &buf)
	defer cleanup()

	req, _ := http.NewRequestWithContext(context.Background(), http.MethodGet, base+"/v1/projects", nil)
	resp, err := client.Do(req)
	if err != nil {
		t.Fatalf("Do: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()
	if _, err := io.ReadAll(resp.Body); err != nil {
		t.Fatalf("read body: %v", err)
	}

	out := buf.String()
	if !strings.Contains(out, "> GET /v1/projects HTTP/1.1") {
		t.Errorf("missing request line:\n%s", out)
	}
	if !strings.Contains(out, "< HTTP/1.1 200 OK") {
		t.Errorf("missing status line:\n%s", out)
	}
	if !strings.Contains(out, `{"ok":true}`) {
		t.Errorf("missing response body:\n%s", out)
	}
}

func TestDebugTransportRedactsAuthorization(t *testing.T) {
	var buf bytes.Buffer
	client, base, cleanup := newDebugClient(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}, &buf)
	defer cleanup()

	req, _ := http.NewRequestWithContext(context.Background(), http.MethodGet, base+"/v1/x", nil)
	req.Header.Set("Authorization", "Bearer ak_abcdefghij1234567890wxyz")
	if _, err := client.Do(req); err != nil {
		t.Fatalf("Do: %v", err)
	}

	out := buf.String()
	if strings.Contains(out, "ak_abcdefghij1234567890wxyz") {
		t.Errorf("full bearer token leaked:\n%s", out)
	}
	if !strings.Contains(out, "Authorization: Bearer ak_a***wxyz") {
		t.Errorf("expected redacted bearer prefix/suffix:\n%s", out)
	}
}

func TestDebugTransportRedactsShortAuthorization(t *testing.T) {
	var buf bytes.Buffer
	client, base, cleanup := newDebugClient(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}, &buf)
	defer cleanup()

	req, _ := http.NewRequestWithContext(context.Background(), http.MethodGet, base+"/v1/x", nil)
	req.Header.Set("Authorization", "Bearer short")
	if _, err := client.Do(req); err != nil {
		t.Fatalf("Do: %v", err)
	}

	out := buf.String()
	if strings.Contains(out, "Bearer short") {
		t.Errorf("short bearer token leaked:\n%s", out)
	}
	if !strings.Contains(out, "Authorization: Bearer ***") {
		t.Errorf("expected collapsed short-token redaction:\n%s", out)
	}
}

func TestDebugTransportDumpsBodyUnderCap(t *testing.T) {
	var buf bytes.Buffer
	body := `{"name":"alpha","count":3}`
	client, base, cleanup := newDebugClient(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"id":"x"}`))
	}, &buf)
	defer cleanup()

	req, _ := http.NewRequestWithContext(context.Background(),
		http.MethodPost, base+"/v1/projects", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	resp, err := client.Do(req)
	if err != nil {
		t.Fatalf("Do: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()
	got, _ := io.ReadAll(resp.Body)
	if string(got) != `{"id":"x"}` {
		t.Errorf("body altered by debug transport: %q", got)
	}

	out := buf.String()
	if !strings.Contains(out, body) {
		t.Errorf("request body not dumped verbatim:\n%s", out)
	}
	if !strings.Contains(out, `{"id":"x"}`) {
		t.Errorf("response body not dumped:\n%s", out)
	}
}

func TestDebugTransportElidesLargeBody(t *testing.T) {
	var buf bytes.Buffer
	big := strings.Repeat("a", debugBodyCap+1024)
	client, base, cleanup := newDebugClient(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = fmt.Fprint(w, big)
	}, &buf)
	defer cleanup()

	req, _ := http.NewRequestWithContext(context.Background(), http.MethodGet, base+"/v1/x", nil)
	resp, err := client.Do(req)
	if err != nil {
		t.Fatalf("Do: %v", err)
	}
	got, _ := io.ReadAll(resp.Body)
	_ = resp.Body.Close()
	if len(got) != len(big) {
		t.Errorf("client received %d bytes, want %d", len(got), len(big))
	}

	out := buf.String()
	if !strings.Contains(out, "1024 bytes elided") {
		t.Errorf("missing elision marker for oversize body:\n%s", out[len(out)-200:])
	}
}

func TestDebugTransportSummarizesBinaryBody(t *testing.T) {
	var buf bytes.Buffer
	payload := bytes.Repeat([]byte{0x00, 0xff}, 256) // 512 bytes
	client, base, cleanup := newDebugClient(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}, &buf)
	defer cleanup()

	req, _ := http.NewRequestWithContext(context.Background(),
		http.MethodPut, base+"/v1/files/upload", bytes.NewReader(payload))
	req.Header.Set("Content-Type", "application/octet-stream")
	if _, err := client.Do(req); err != nil {
		t.Fatalf("Do: %v", err)
	}

	out := buf.String()
	if strings.Contains(out, string(payload)) {
		t.Errorf("binary body leaked into debug output")
	}
	if !strings.Contains(out, "(512 bytes of application/octet-stream)") {
		t.Errorf("missing binary-body summary:\n%s", out)
	}
}

func TestDebugTransportBodyTypePreserved(t *testing.T) {
	// Sanity check: an octet-stream upload's payload must reach the server
	// intact even though the debug transport drained and replaced the body.
	var got []byte
	var buf bytes.Buffer
	payload := []byte{0x01, 0x02, 0x03, 0x04, 0x05}
	client, base, cleanup := newDebugClient(t, func(w http.ResponseWriter, r *http.Request) {
		got, _ = io.ReadAll(r.Body)
		w.WriteHeader(http.StatusOK)
	}, &buf)
	defer cleanup()

	req, _ := http.NewRequestWithContext(context.Background(),
		http.MethodPut, base+"/v1/files/x", bytes.NewReader(payload))
	req.Header.Set("Content-Type", "application/octet-stream")
	if _, err := client.Do(req); err != nil {
		t.Fatalf("Do: %v", err)
	}
	if !bytes.Equal(got, payload) {
		t.Errorf("server saw %v, want %v", got, payload)
	}
}

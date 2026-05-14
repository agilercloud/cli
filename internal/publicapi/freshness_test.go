package publicapi_test

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// TestPublicAPIClientIsFresh regenerates the Go client from public.json and
// diffs it against the committed client.gen.go. If someone updates the spec
// (in the sibling platform repo) without running `make openapi`, this test
// fails — preventing the committed client from drifting from the contract.
//
// The spec lives in a separate repo at ../../../../platform/api/openapi/
// public.json, expected to be a sibling checkout. When the platform sibling
// is not present (e.g. CI for a distributed CLI tarball that ships without
// the platform tree), the test skips with a clear message.
func TestPublicAPIClientIsFresh(t *testing.T) {
	if _, err := exec.LookPath("go"); err != nil {
		t.Skip("go binary not on PATH; skipping client freshness test")
	}

	specPath, err := filepath.Abs(filepath.Join("..", "..", "..", "platform", "api", "openapi", "public.json"))
	if err != nil {
		t.Fatalf("resolve spec path: %v", err)
	}
	if _, err := os.Stat(specPath); err != nil {
		t.Skipf("platform sibling not checked out at %s; skipping client freshness test", specPath)
	}
	configPath, err := filepath.Abs("gen.yaml")
	if err != nil {
		t.Fatalf("resolve config path: %v", err)
	}
	committedPath, err := filepath.Abs("client.gen.go")
	if err != nil {
		t.Fatalf("resolve committed client path: %v", err)
	}

	tmp := t.TempDir()
	cmd := exec.Command("go", "run", "github.com/oapi-codegen/oapi-codegen/v2/cmd/oapi-codegen@v2.7.0",
		"-config", configPath,
		specPath,
	)
	cmd.Dir = tmp // gen.yaml has `output: client.gen.go` (relative) — run from tmp so the artifact lands there.
	cmd.Env = append(os.Environ(), "GOFLAGS=-mod=mod")
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		t.Fatalf("oapi-codegen: %v\n%s", err, stderr.String())
	}

	want, err := os.ReadFile(committedPath)
	if err != nil {
		t.Fatalf("read committed client: %v", err)
	}
	got, err := os.ReadFile(filepath.Join(tmp, "client.gen.go"))
	if err != nil {
		t.Fatalf("read fresh client: %v", err)
	}
	if !bytes.Equal(want, got) {
		t.Fatalf("committed Go client for public.json is out of date.\n\n"+
			"Run `make openapi` and commit the result. The route table in\n"+
			"the platform repo at internal/api/api/v1/register.go (or one of\n"+
			"its referenced types) has changed without a matching regeneration\n"+
			"of the Go client.\n\nFirst diff: %s", firstDiff(want, got))
	}
}

// firstDiff returns a small excerpt at the first byte that differs between
// a and b, useful for surfacing the source of staleness without dumping a
// 600-line diff into test output.
func firstDiff(a, b []byte) string {
	n := min(len(a), len(b))
	for i := range n {
		if a[i] != b[i] {
			start := max(i-40, 0)
			end := min(i+40, n)
			return fmt.Sprintf("byte %d: committed=%q / fresh=%q",
				i,
				strings.TrimSpace(string(a[start:end])),
				strings.TrimSpace(string(b[start:end])),
			)
		}
	}
	return fmt.Sprintf("length mismatch (committed=%d, fresh=%d)", len(a), len(b))
}

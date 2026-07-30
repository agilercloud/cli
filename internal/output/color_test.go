package output

import (
	"bytes"
	"os"
	"testing"
)

func TestNewColorForceOff(t *testing.T) {
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = r.Close() }()
	defer func() { _ = w.Close() }()
	c := NewColor(w, true)
	if c.Enabled() {
		t.Errorf("forceOff=true should yield disabled color")
	}
}

func TestNewColorNonFileWriter(t *testing.T) {
	c := NewColor(&bytes.Buffer{}, false)
	if c.Enabled() {
		t.Errorf("non-*os.File writer should yield disabled color, got enabled")
	}
}

func TestNewColorPipeNotTTY(t *testing.T) {
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = r.Close() }()
	defer func() { _ = w.Close() }()
	c := NewColor(w, false)
	if c.Enabled() {
		t.Errorf("os.Pipe write end is not a TTY; color should be disabled")
	}
}

func TestNewColorNoColorEnv(t *testing.T) {
	// Even on a TTY, NO_COLOR forces disable. We test the path here by
	// setting NO_COLOR and passing any *os.File — Stat will succeed but the
	// env check short-circuits earlier.
	t.Setenv("NO_COLOR", "1")
	c := NewColor(os.Stdout, false)
	if c.Enabled() {
		t.Errorf("NO_COLOR set should disable color regardless of TTY")
	}
}

func TestColorEnabledWrapsCodes(t *testing.T) {
	c := Color{enabled: true}
	cases := []struct {
		name string
		got  string
		want string
	}{
		{"red", c.Red("x"), "\x1b[31mx\x1b[0m"},
		{"green", c.Green("x"), "\x1b[32mx\x1b[0m"},
		{"yellow", c.Yellow("x"), "\x1b[33mx\x1b[0m"},
		{"bold", c.Bold("x"), "\x1b[1mx\x1b[0m"},
		{"dim", c.Dim("x"), "\x1b[2mx\x1b[0m"},
	}
	for _, tc := range cases {
		if tc.got != tc.want {
			t.Errorf("%s: got %q, want %q", tc.name, tc.got, tc.want)
		}
	}
}

func TestColorDisabledPassesThrough(t *testing.T) {
	c := Color{}
	if got := c.Red("x"); got != "x" {
		t.Errorf("disabled Red: got %q, want %q", got, "x")
	}
	if got := c.Bold("hello"); got != "hello" {
		t.Errorf("disabled Bold: got %q, want %q", got, "hello")
	}
}

func TestColorEmptyStringNeverWrapped(t *testing.T) {
	c := Color{enabled: true}
	if got := c.Red(""); got != "" {
		t.Errorf("empty input should never be wrapped, got %q", got)
	}
	if got := c.Bold(""); got != "" {
		t.Errorf("empty input should never be wrapped, got %q", got)
	}
}

func TestBoldHeaderTokens(t *testing.T) {
	c := Color{enabled: true}
	got := boldHeaderTokens("ID  NAME    STATUS", c)
	want := "\x1b[1mID\x1b[0m  \x1b[1mNAME\x1b[0m    \x1b[1mSTATUS\x1b[0m"
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestBoldHeaderTokensDisabled(t *testing.T) {
	got := boldHeaderTokens("ID  NAME    STATUS", Color{})
	if got != "ID  NAME    STATUS" {
		t.Errorf("disabled Color should leave string unchanged, got %q", got)
	}
}

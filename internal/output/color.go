package output

import (
	"io"
	"os"
)

const (
	ansiReset  = "\x1b[0m"
	ansiBold   = "\x1b[1m"
	ansiDim    = "\x1b[2m"
	ansiRed    = "\x1b[31m"
	ansiGreen  = "\x1b[32m"
	ansiYellow = "\x1b[33m"
)

// Color decides whether to emit ANSI escape codes for a particular writer.
// The zero value is a no-op renderer; constructed via NewColor, it enables
// codes only when the writer is a TTY, NO_COLOR is unset, and forceOff is
// false.
type Color struct {
	enabled bool
}

// NewColor returns a Color that emits escape codes only when w is a TTY,
// NO_COLOR is unset, and forceOff is false. forceOff is set by callers that
// know color is inappropriate regardless of the writer (structured output,
// quiet mode, --no-color).
func NewColor(w io.Writer, forceOff bool) Color {
	if forceOff {
		return Color{}
	}
	if os.Getenv("NO_COLOR") != "" {
		return Color{}
	}
	f, ok := w.(*os.File)
	if !ok {
		return Color{}
	}
	fi, err := f.Stat()
	if err != nil {
		return Color{}
	}
	if fi.Mode()&os.ModeCharDevice == 0 {
		return Color{}
	}
	return Color{enabled: true}
}

// Enabled reports whether escape codes will be emitted.
func (c Color) Enabled() bool { return c.enabled }

func (c Color) wrap(code, s string) string {
	if !c.enabled || s == "" {
		return s
	}
	return code + s + ansiReset
}

func (c Color) Red(s string) string    { return c.wrap(ansiRed, s) }
func (c Color) Green(s string) string  { return c.wrap(ansiGreen, s) }
func (c Color) Yellow(s string) string { return c.wrap(ansiYellow, s) }
func (c Color) Bold(s string) string   { return c.wrap(ansiBold, s) }
func (c Color) Dim(s string) string    { return c.wrap(ansiDim, s) }

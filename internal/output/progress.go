package output

import (
	"fmt"
	"io"
	"sync/atomic"
	"time"
)

// ProgressReader wraps a reader and writes a single-line, CR-overwritten
// progress ticker to W. Total may be 0 if Content-Length is unknown, in
// which case the percentage is omitted from the rendered line.
type ProgressReader struct {
	R     io.Reader
	W     io.Writer
	Label string
	Total int64
	Color Color

	start       time.Time
	bytes       int64
	lastTick    time.Time
	lastWidth   int
	minInterval time.Duration
	done        bool
}

// NewProgressReader returns a ProgressReader that renders to w. The renderer
// throttles updates to roughly ten frames per second; callers should invoke
// Finish exactly once after the underlying copy completes.
func NewProgressReader(r io.Reader, w io.Writer, label string, total int64, color Color) *ProgressReader {
	return &ProgressReader{
		R:           r,
		W:           w,
		Label:       label,
		Total:       total,
		Color:       color,
		start:       time.Now(),
		minInterval: 100 * time.Millisecond,
	}
}

func (p *ProgressReader) Read(buf []byte) (int, error) {
	n, err := p.R.Read(buf)
	if n > 0 {
		atomic.AddInt64(&p.bytes, int64(n))
	}
	now := time.Now()
	if now.Sub(p.lastTick) >= p.minInterval || err != nil {
		p.render(now, false)
		p.lastTick = now
	}
	return n, err
}

// Finish writes the final line (with a "done" suffix on success) and
// terminates it with a newline. Subsequent calls are no-ops.
func (p *ProgressReader) Finish(ok bool) {
	if p.done {
		return
	}
	p.done = true
	p.render(time.Now(), ok)
	_, _ = fmt.Fprintln(p.W)
}

func (p *ProgressReader) render(now time.Time, final bool) {
	elapsed := now.Sub(p.start)
	n := atomic.LoadInt64(&p.bytes)
	var rate float64
	if elapsed >= time.Millisecond {
		rate = float64(n) / elapsed.Seconds()
	}
	var pct string
	if p.Total > 0 {
		pct = fmt.Sprintf("  %3.0f%%", 100*float64(n)/float64(p.Total))
	}
	suffix := ""
	if final {
		suffix = "  done"
	}
	body := fmt.Sprintf("%s  %s  %s/s  %s%s%s", p.Label, formatBytes(n), formatBytes(int64(rate)), formatDuration(elapsed), pct, suffix)
	pad := ""
	if d := p.lastWidth - len(body); d > 0 {
		pad = padSpaces(d)
	}
	p.lastWidth = len(body)
	_, _ = fmt.Fprintf(p.W, "\r%s%s", body, pad)
}

func padSpaces(n int) string {
	const spaces = "                                                                "
	if n <= len(spaces) {
		return spaces[:n]
	}
	buf := make([]byte, n)
	for i := range buf {
		buf[i] = ' '
	}
	return string(buf)
}

func formatBytes(n int64) string {
	const (
		kb = 1 << 10
		mb = 1 << 20
		gb = 1 << 30
	)
	switch {
	case n >= gb:
		return fmt.Sprintf("%.1f GB", float64(n)/gb)
	case n >= mb:
		return fmt.Sprintf("%.1f MB", float64(n)/mb)
	case n >= kb:
		return fmt.Sprintf("%.1f KB", float64(n)/kb)
	default:
		return fmt.Sprintf("%d B", n)
	}
}

func formatDuration(d time.Duration) string {
	switch {
	case d < time.Second:
		return fmt.Sprintf("%dms", d.Milliseconds())
	case d < time.Minute:
		return fmt.Sprintf("%.1fs", d.Seconds())
	default:
		return fmt.Sprintf("%dm%ds", int(d.Minutes()), int(d.Seconds())%60)
	}
}

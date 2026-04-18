// Package output renders CLI command results in one of three modes:
// text (human-readable tables), JSON (structured), or quiet (IDs only).
package output

import (
	"encoding/json"
	"fmt"
	"io"
	"strings"
	"text/tabwriter"
)

// Mode controls how a Writer renders results.
type Mode int

const (
	ModeText Mode = iota
	ModeJSON
	ModeQuiet
)

// Writer is the per-command output sink. Callers configure Mode and the
// underlying streams once at startup; methods take care of the per-mode
// formatting.
type Writer struct {
	Mode Mode
	Out  io.Writer
	Err  io.Writer
}

// New constructs a Writer for the given mode and streams.
func New(mode Mode, stdout, stderr io.Writer) *Writer {
	return &Writer{Mode: mode, Out: stdout, Err: stderr}
}

// IsJSON reports whether the writer is in JSON mode.
// Renderers that need a structurally different layout between text and JSON
// (e.g. "detail" views) may branch on this; list renderers shouldn't.
func (w *Writer) IsJSON() bool { return w.Mode == ModeJSON }

// IsQuiet reports whether the writer is in Quiet mode.
func (w *Writer) IsQuiet() bool { return w.Mode == ModeQuiet }

// Table renders rows as a text table, a JSON array of objects keyed by
// headers, or just the first column per row (for Quiet mode).
func (w *Writer) Table(headers []string, rows [][]string) {
	switch w.Mode {
	case ModeJSON:
		w.JSON(tableToObjects(headers, rows))
	case ModeQuiet:
		for _, row := range rows {
			if len(row) > 0 {
				fmt.Fprintln(w.Out, row[0])
			}
		}
	default:
		tw := tabwriter.NewWriter(w.Out, 0, 0, 2, ' ', 0)
		fmt.Fprintln(tw, strings.Join(headers, "\t"))
		for _, row := range rows {
			fmt.Fprintln(tw, strings.Join(row, "\t"))
		}
		tw.Flush()
	}
}

// Text writes a formatted line to stdout. Always prints regardless of mode;
// use this for status messages like "Project created." — callers that want
// the message suppressed in JSON mode should guard with IsJSON().
func (w *Writer) Text(format string, args ...any) {
	fmt.Fprintf(w.Out, format+"\n", args...)
}

// JSON encodes v as indented JSON to stdout.
func (w *Writer) JSON(v any) {
	enc := json.NewEncoder(w.Out)
	enc.SetIndent("", "  ")
	_ = enc.Encode(v)
}

// RawJSON copies already-serialized JSON bytes straight to stdout.
func (w *Writer) RawJSON(r io.Reader) {
	_, _ = io.Copy(w.Out, r)
	fmt.Fprintln(w.Out)
}

// Stderr writes a diagnostic line to stderr (progress, warnings).
func (w *Writer) Stderr(format string, args ...any) {
	fmt.Fprintf(w.Err, format+"\n", args...)
}

func tableToObjects(headers []string, rows [][]string) []map[string]string {
	out := make([]map[string]string, len(rows))
	for i, r := range rows {
		m := make(map[string]string, len(headers))
		for j, h := range headers {
			if j < len(r) {
				m[h] = r[j]
			}
		}
		out[i] = m
	}
	return out
}

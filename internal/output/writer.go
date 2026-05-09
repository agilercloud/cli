// Package output renders CLI command results in one of several formats:
// text (human-readable tables), JSON, YAML, CSV, or TSV. The Quiet flag
// composes orthogonally and trims output to first-column / id-only form.
package output

import (
	"encoding/csv"
	"encoding/json"
	"fmt"
	"io"
	"strings"
	"text/tabwriter"

	"sigs.k8s.io/yaml"
)

// Format controls how a Writer renders results.
type Format string

const (
	FormatText Format = "text"
	FormatJSON Format = "json"
	FormatYAML Format = "yaml"
	FormatCSV  Format = "csv"
	FormatTSV  Format = "tsv"
)

// ParseFormat converts a --format flag value into a Format. Empty string
// is rejected; callers should default to FormatText themselves before calling.
func ParseFormat(s string) (Format, error) {
	switch Format(s) {
	case FormatText, FormatJSON, FormatYAML, FormatCSV, FormatTSV:
		return Format(s), nil
	}
	return "", fmt.Errorf("invalid format %q: must be one of text, json, yaml, csv, tsv", s)
}

// Writer is the per-command output sink. Format and Quiet are configured
// once at startup; methods take care of the per-format dispatch.
type Writer struct {
	Format Format
	Quiet  bool
	Out    io.Writer
	Err    io.Writer
}

// New constructs a Writer for the given format and quiet flag.
func New(format Format, quiet bool, stdout, stderr io.Writer) *Writer {
	return &Writer{Format: format, Quiet: quiet, Out: stdout, Err: stderr}
}

// IsStructured reports whether the writer emits a single structured document
// per result (json or yaml). Detail-view callers branch on this to choose
// between Structured() and hand-formatted text.
func (w *Writer) IsStructured() bool {
	return w.Format == FormatJSON || w.Format == FormatYAML
}

// IsTabular reports whether the writer requires tabular row/column output
// (csv or tsv). Detail-view callers reject these formats with an error.
func (w *Writer) IsTabular() bool {
	return w.Format == FormatCSV || w.Format == FormatTSV
}

// IsQuiet reports whether quiet mode is active.
func (w *Writer) IsQuiet() bool { return w.Quiet }

// Table renders rows in the configured format. With Quiet, only the first
// column is emitted (and headers are suppressed for csv/tsv).
func (w *Writer) Table(headers []string, rows [][]string) {
	switch w.Format {
	case FormatJSON, FormatYAML:
		if w.Quiet {
			w.Structured(firstColumn(rows))
			return
		}
		w.Structured(tableToObjects(headers, rows))
	case FormatCSV:
		w.writeDelimited(headers, rows, ',')
	case FormatTSV:
		w.writeDelimited(headers, rows, '\t')
	default:
		if w.Quiet {
			for _, row := range rows {
				if len(row) > 0 {
					_, _ = fmt.Fprintln(w.Out, row[0])
				}
			}
			return
		}
		tw := tabwriter.NewWriter(w.Out, 0, 0, 2, ' ', 0)
		_, _ = fmt.Fprintln(tw, strings.Join(headers, "\t"))
		for _, row := range rows {
			_, _ = fmt.Fprintln(tw, strings.Join(row, "\t"))
		}
		_ = tw.Flush()
	}
}

// Text writes a formatted line to stdout. Always prints regardless of format;
// callers that want the message suppressed in structured modes should guard
// with IsStructured()/IsTabular().
func (w *Writer) Text(format string, args ...any) {
	_, _ = fmt.Fprintf(w.Out, format+"\n", args...)
}

// Structured encodes v as JSON or YAML to stdout, depending on Format.
// For text/csv/tsv this is a no-op; callers should gate with IsStructured().
func (w *Writer) Structured(v any) {
	switch w.Format {
	case FormatJSON:
		enc := json.NewEncoder(w.Out)
		enc.SetIndent("", "  ")
		_ = enc.Encode(v)
	case FormatYAML:
		b, err := yaml.Marshal(v)
		if err != nil {
			_, _ = fmt.Fprintf(w.Err, "yaml encode: %v\n", err)
			return
		}
		_, _ = w.Out.Write(b)
	}
}

// JSON encodes v in the configured structured format. Used by sites that
// always emit a server-defined structured payload (sql results, rule
// templates) regardless of caller layout. For csv/tsv it warns on stderr
// since arbitrary nested data isn't tabular.
func (w *Writer) JSON(v any) {
	if w.IsTabular() {
		_, _ = fmt.Fprintf(w.Err, "warning: --format=%s is not supported here; emitting json\n", w.Format)
		enc := json.NewEncoder(w.Out)
		enc.SetIndent("", "  ")
		_ = enc.Encode(v)
		return
	}
	if w.Format == FormatYAML {
		w.Structured(v)
		return
	}
	enc := json.NewEncoder(w.Out)
	enc.SetIndent("", "  ")
	_ = enc.Encode(v)
}

// RawJSON copies already-serialized JSON bytes straight to stdout.
func (w *Writer) RawJSON(r io.Reader) {
	_, _ = io.Copy(w.Out, r)
	_, _ = fmt.Fprintln(w.Out)
}

// Stderr writes a diagnostic line to stderr (progress, warnings).
func (w *Writer) Stderr(format string, args ...any) {
	_, _ = fmt.Fprintf(w.Err, format+"\n", args...)
}

func (w *Writer) writeDelimited(headers []string, rows [][]string, comma rune) {
	cw := csv.NewWriter(w.Out)
	cw.Comma = comma
	if w.Quiet {
		for _, row := range rows {
			if len(row) > 0 {
				_ = cw.Write(row[:1])
			}
		}
		cw.Flush()
		return
	}
	_ = cw.Write(headers)
	for _, row := range rows {
		_ = cw.Write(row)
	}
	cw.Flush()
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

func firstColumn(rows [][]string) []string {
	out := make([]string, 0, len(rows))
	for _, r := range rows {
		if len(r) > 0 {
			out = append(out, r[0])
		}
	}
	return out
}

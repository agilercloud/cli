package cli

import "github.com/agilercloud/cli/internal/output"

// renderTable renders a homogeneous list as structured output or a table.
func renderTable[T any](w *output.Writer, items []T, emptyMessage string, headers []string, row func(T) []string) {
	if w.IsStructured() {
		w.Structured(items)
		return
	}
	if len(items) == 0 {
		w.Text("%s", emptyMessage)
		return
	}
	rows := make([][]string, len(items))
	for i, item := range items {
		rows[i] = row(item)
	}
	w.Table(headers, rows)
}

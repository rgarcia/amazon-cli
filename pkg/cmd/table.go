package cmd

import (
	"fmt"
	"io"
	"os"
	"strconv"

	"golang.org/x/term"
)

const columnGap = 2

type TableWriter struct {
	w          io.Writer
	headers    []string
	widths     []int
	rows       [][]string
	TruncOrder []int
}

func NewTableWriter(w io.Writer, headers ...string) *TableWriter {
	widths := make([]int, len(headers))
	for i, h := range headers {
		widths[i] = len(h)
	}
	return &TableWriter{w: w, headers: headers, widths: widths}
}

func (t *TableWriter) AddRow(cells ...string) {
	row := make([]string, len(t.headers))
	for i := range row {
		if i < len(cells) {
			row[i] = cells[i]
		}
		if len(row[i]) > t.widths[i] {
			t.widths[i] = len(row[i])
		}
	}
	t.rows = append(t.rows, row)
}

func getTerminalWidth() int {
	if w, _, err := term.GetSize(int(os.Stdout.Fd())); err == nil && w > 0 {
		return w
	}
	if w, _, err := term.GetSize(int(os.Stderr.Fd())); err == nil && w > 0 {
		return w
	}
	if cols := os.Getenv("COLUMNS"); cols != "" {
		if w, err := strconv.Atoi(cols); err == nil && w > 0 {
			return w
		}
	}
	return 120
}

func (t *TableWriter) renderWidths() []int {
	n := len(t.headers)
	widths := make([]int, n)
	copy(widths, t.widths)
	total := func() int {
		sum := columnGap * (n - 1)
		for _, w := range widths {
			sum += w
		}
		return sum
	}
	for _, col := range t.TruncOrder {
		if col < 0 || col >= n {
			continue
		}
		excess := total() - getTerminalWidth()
		if excess <= 0 {
			break
		}
		minW := len(t.headers[col])
		if minW < 5 {
			minW = 5
		}
		canShrink := widths[col] - minW
		if canShrink <= 0 {
			continue
		}
		if excess > canShrink {
			excess = canShrink
		}
		widths[col] -= excess
	}
	return widths
}

func (t *TableWriter) Render() {
	widths := t.renderWidths()
	last := len(t.headers) - 1
	for i, h := range t.headers {
		cell := truncateCell(h, widths[i])
		if i < last {
			fmt.Fprintf(t.w, "%-*s", widths[i]+columnGap, cell)
		} else {
			fmt.Fprint(t.w, cell)
		}
	}
	fmt.Fprintln(t.w)
	for _, row := range t.rows {
		for i, cell := range row {
			cell = truncateCell(cell, widths[i])
			if i < last {
				fmt.Fprintf(t.w, "%-*s", widths[i]+columnGap, cell)
			} else {
				fmt.Fprint(t.w, cell)
			}
		}
		fmt.Fprintln(t.w)
	}
}

func truncateCell(s string, maxWidth int) string {
	if len(s) <= maxWidth {
		return s
	}
	if maxWidth <= 3 {
		return s[:maxWidth]
	}
	return s[:maxWidth-3] + "..."
}

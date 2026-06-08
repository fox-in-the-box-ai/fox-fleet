package output

import (
	"encoding/json"
	"fmt"
	"io"
	"strings"
	"text/tabwriter"
)

type Format string

const (
	Table Format = "table"
	JSON  Format = "json"
	Quiet Format = "quiet"
)

func ParseFormat(s string) (Format, error) {
	switch strings.ToLower(s) {
	case "table", "":
		return Table, nil
	case "json":
		return JSON, nil
	case "quiet":
		return Quiet, nil
	default:
		return "", fmt.Errorf("invalid output format %q — use table, json, or quiet", s)
	}
}

type Writer struct {
	format Format
	out    io.Writer
}

func NewWriter(out io.Writer, f Format) *Writer {
	return &Writer{format: f, out: out}
}

func (w *Writer) WriteTable(headers []string, rows [][]string) error {
	switch w.format {
	case JSON:
		result := make([]map[string]string, len(rows))
		for i, row := range rows {
			m := make(map[string]string, len(headers))
			for j, h := range headers {
				if j < len(row) {
					m[strings.ToLower(h)] = row[j]
				}
			}
			result[i] = m
		}
		enc := json.NewEncoder(w.out)
		enc.SetIndent("", "  ")
		return enc.Encode(result)
	case Quiet:
		for _, row := range rows {
			if len(row) > 0 {
				fmt.Fprintln(w.out, row[0])
			}
		}
		return nil
	default:
		tw := tabwriter.NewWriter(w.out, 0, 4, 2, ' ', 0)
		fmt.Fprintln(tw, strings.Join(headers, "\t"))
		for _, row := range rows {
			fmt.Fprintln(tw, strings.Join(row, "\t"))
		}
		return tw.Flush()
	}
}

func (w *Writer) WriteJSON(v any) error {
	enc := json.NewEncoder(w.out)
	enc.SetIndent("", "  ")
	return enc.Encode(v)
}

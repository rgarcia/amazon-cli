package cmd

import (
	"fmt"
	"io"
	"strings"
	"unicode"

	"github.com/tidwall/gjson"
)

type detailField struct {
	key string
	val gjson.Result
}

var knownKeyAbbreviations = map[string]string{
	"id":   "ID",
	"url":  "URL",
	"api":  "API",
	"html": "HTML",
	"asin": "ASIN",
}

func ShowDetail(w io.Writer, data gjson.Result) {
	colors := shouldUseColors(w)
	if !data.IsObject() {
		fmt.Fprintln(w, formatDetailValue(data))
		return
	}

	var scalars []detailField
	var sections []detailField
	data.ForEach(func(key, val gjson.Result) bool {
		field := detailField{key: key.String(), val: val}
		if val.IsObject() || val.IsArray() {
			sections = append(sections, field)
		} else {
			scalars = append(scalars, field)
		}
		return true
	})

	if len(scalars) > 0 {
		renderDetailKVBlock(w, "", scalars, colors)
	}
	for _, section := range sections {
		renderDetailSection(w, section.key, section.val, 0, colors)
	}
}

func renderDetailKVBlock(w io.Writer, header string, fields []detailField, colors bool) {
	indent := ""
	if header != "" {
		fmt.Fprintf(w, "\n%s\n", styledDetailHeader(header, colors))
		indent = "  "
	}

	maxKeyLen := 0
	for _, f := range fields {
		label := detailTitle(f.key)
		if len(label) > maxKeyLen {
			maxKeyLen = len(label)
		}
	}
	for _, f := range fields {
		label := detailTitle(f.key)
		fmt.Fprintf(w, "%s%-*s  %s\n", indent, maxKeyLen, label, formatDetailValue(f.val))
	}
}

func renderDetailSection(w io.Writer, name string, val gjson.Result, depth int, colors bool) {
	indent := strings.Repeat("  ", depth)
	fmt.Fprintf(w, "\n%s%s\n", indent, styledDetailHeader(name, colors))

	if val.IsArray() {
		renderDetailArray(w, val.Array(), depth, colors)
		return
	}
	if !val.IsObject() {
		fmt.Fprintf(w, "%s  %s\n", indent, formatDetailValue(val))
		return
	}

	var scalars []detailField
	var nested []detailField
	val.ForEach(func(key, child gjson.Result) bool {
		field := detailField{key: key.String(), val: child}
		if child.IsObject() || child.IsArray() {
			nested = append(nested, field)
		} else {
			scalars = append(scalars, field)
		}
		return true
	})
	if len(scalars) > 0 {
		renderDetailKVRows(w, scalars, depth+1)
	}
	for _, child := range nested {
		renderDetailSection(w, child.key, child.val, depth+1, colors)
	}
}

func renderDetailArray(w io.Writer, arr []gjson.Result, depth int, colors bool) {
	indent := strings.Repeat("  ", depth)
	if len(arr) == 0 {
		fmt.Fprintf(w, "%s  (none)\n", indent)
		return
	}
	if arr[0].IsObject() {
		renderDetailObjectArray(w, arr, depth, colors)
		return
	}
	for _, item := range arr {
		fmt.Fprintf(w, "%s  - %s\n", indent, formatDetailValue(item))
	}
}

func renderDetailObjectArray(w io.Writer, arr []gjson.Result, depth int, colors bool) {
	indent := strings.Repeat("  ", depth)
	var keys []string
	keySet := make(map[string]bool)
	hasNested := false

	for _, item := range arr {
		item.ForEach(func(key, val gjson.Result) bool {
			k := key.String()
			if !keySet[k] {
				keySet[k] = true
				keys = append(keys, k)
			}
			if val.IsObject() || val.IsArray() {
				hasNested = true
			}
			return true
		})
	}

	if !hasNested && len(keys) <= 6 {
		renderDetailTable(w, arr, keys, depth)
		return
	}

	for i, item := range arr {
		if i > 0 {
			fmt.Fprintf(w, "%s  ---\n", indent)
		}
		var scalars []detailField
		var nested []detailField
		item.ForEach(func(key, val gjson.Result) bool {
			field := detailField{key: key.String(), val: val}
			if val.IsObject() || val.IsArray() {
				nested = append(nested, field)
			} else {
				scalars = append(scalars, field)
			}
			return true
		})
		renderDetailKVRows(w, scalars, depth+1)
		for _, child := range nested {
			renderDetailSection(w, child.key, child.val, depth+1, colors)
		}
	}
}

func renderDetailTable(w io.Writer, arr []gjson.Result, keys []string, depth int) {
	indent := strings.Repeat("  ", depth)
	widths := make([]int, len(keys))
	for i, key := range keys {
		widths[i] = len(detailTitle(key))
	}
	rows := make([][]string, 0, len(arr))
	for _, item := range arr {
		row := make([]string, len(keys))
		for i, key := range keys {
			row[i] = formatDetailValue(item.Get(key))
			if len(row[i]) > widths[i] {
				widths[i] = len(row[i])
			}
		}
		rows = append(rows, row)
	}

	for i, key := range keys {
		cell := detailTitle(key)
		if i < len(keys)-1 {
			fmt.Fprintf(w, "%s  %-*s", indent, widths[i]+2, cell)
		} else {
			fmt.Fprintf(w, "%s\n", cell)
		}
	}
	for _, row := range rows {
		for i, cell := range row {
			if i < len(row)-1 {
				fmt.Fprintf(w, "%s  %-*s", indent, widths[i]+2, cell)
			} else {
				fmt.Fprintf(w, "%s\n", cell)
			}
		}
	}
}

func renderDetailKVRows(w io.Writer, fields []detailField, depth int) {
	indent := strings.Repeat("  ", depth)
	maxKeyLen := 0
	for _, f := range fields {
		label := detailTitle(f.key)
		if len(label) > maxKeyLen {
			maxKeyLen = len(label)
		}
	}
	for _, f := range fields {
		label := detailTitle(f.key)
		fmt.Fprintf(w, "%s%-*s  %s\n", indent, maxKeyLen, label, formatDetailValue(f.val))
	}
}

func formatDetailValue(v gjson.Result) string {
	switch v.Type {
	case gjson.Null:
		return "-"
	case gjson.True:
		return "yes"
	case gjson.False:
		return "no"
	case gjson.String:
		return v.String()
	case gjson.Number:
		f := v.Float()
		if f == float64(int64(f)) {
			return fmt.Sprintf("%d", int64(f))
		}
		return strings.TrimRight(strings.TrimRight(fmt.Sprintf("%.2f", f), "0"), ".")
	default:
		if v.Raw == "" {
			return "-"
		}
		return v.Raw
	}
}

func styledDetailHeader(name string, colors bool) string {
	title := detailTitle(name)
	if colors {
		return fmt.Sprintf("\033[1m%s\033[0m", title)
	}
	return title
}

func detailTitle(s string) string {
	if s == "" {
		return s
	}
	parts := strings.FieldsFunc(s, func(r rune) bool {
		return r == '_' || r == '-' || r == ' '
	})
	for i, part := range parts {
		parts[i] = camelPartToTitle(part)
	}
	return strings.Join(parts, " ")
}

func camelPartToTitle(s string) string {
	if s == "" {
		return s
	}
	if upper, ok := knownKeyAbbreviations[strings.ToLower(s)]; ok {
		return upper
	}

	var words []string
	var b strings.Builder
	runes := []rune(s)
	for i, r := range runes {
		if i > 0 && unicode.IsUpper(r) {
			prev := runes[i-1]
			nextLower := i+1 < len(runes) && unicode.IsLower(runes[i+1])
			if unicode.IsLower(prev) || nextLower {
				words = append(words, b.String())
				b.Reset()
			}
		}
		b.WriteRune(r)
	}
	words = append(words, b.String())

	for i, word := range words {
		lower := strings.ToLower(word)
		if upper, ok := knownKeyAbbreviations[lower]; ok {
			words[i] = upper
			continue
		}
		rs := []rune(lower)
		if len(rs) > 0 {
			rs[0] = unicode.ToUpper(rs[0])
		}
		words[i] = string(rs)
	}
	return strings.Join(words, " ")
}

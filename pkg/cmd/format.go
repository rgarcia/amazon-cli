package cmd

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/itchyny/json2yaml"
	"github.com/tidwall/gjson"
	"github.com/tidwall/pretty"
	"golang.org/x/term"
)

var OutputFormats = []string{"auto", "json", "jsonline", "pretty", "raw", "yaml"}

func isTerminal(w io.Writer) bool {
	if f, ok := w.(*os.File); ok {
		return term.IsTerminal(int(f.Fd()))
	}
	return false
}

func shouldUseColors(w io.Writer) bool {
	if force, ok := os.LookupEnv("FORCE_COLOR"); ok {
		return force == "1"
	}
	return isTerminal(w)
}

func ShowJSON(out io.Writer, raw string, format string, transform string) error {
	res := gjson.Parse(raw)
	if transform != "" {
		if transformed := res.Get(transform); transformed.Exists() {
			res = transformed
		}
	}
	switch strings.ToLower(format) {
	case "auto":
		ShowDetail(out, res)
		return nil
	case "json":
		prettyJSON := pretty.Pretty([]byte(res.Raw))
		if shouldUseColors(out) {
			_, err := out.Write(pretty.Color(prettyJSON, pretty.TerminalStyle))
			return err
		}
		_, err := out.Write(prettyJSON)
		return err
	case "pretty":
		_, err := out.Write(pretty.Pretty([]byte(res.Raw)))
		return err
	case "jsonline", "raw":
		_, err := out.Write([]byte(res.Raw + "\n"))
		return err
	case "yaml":
		input := strings.NewReader(res.Raw)
		var yamlOut strings.Builder
		if err := json2yaml.Convert(&yamlOut, input); err != nil {
			return err
		}
		_, err := out.Write([]byte(yamlOut.String()))
		return err
	default:
		return fmt.Errorf("invalid format: %s, valid formats are: %s", format, strings.Join(OutputFormats, ", "))
	}
}

func ShowAny(out io.Writer, data any, format string, transform string) error {
	b, err := json.Marshal(data)
	if err != nil {
		return err
	}
	return ShowJSON(out, string(b), format, transform)
}

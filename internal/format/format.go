// Package format defines the set of formats fx can convert between and
// provides helpers to identify a format from a file extension or from raw
// bytes.
package format

import (
	"bytes"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/m-mizutani/goerr/v2"
)

// Format identifies a supported serialization format.
type Format string

const (
	FormatJSON    Format = "json"
	FormatYAML    Format = "yaml"
	FormatTOML    Format = "toml"
	FormatHCL     Format = "hcl"
	FormatJsonnet Format = "jsonnet"
)

// All returns every supported Format in a stable order.
// Used for help text and parameter validation.
func All() []Format {
	return []Format{FormatJSON, FormatYAML, FormatTOML, FormatHCL, FormatJsonnet}
}

// Parse normalises a user-supplied format string (e.g. via the --src flag) to
// a Format value. Names are case-insensitive; common aliases are accepted.
func Parse(s string) (Format, error) {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "json":
		return FormatJSON, nil
	case "yaml", "yml":
		return FormatYAML, nil
	case "toml":
		return FormatTOML, nil
	case "hcl":
		return FormatHCL, nil
	case "jsonnet", "libsonnet":
		return FormatJsonnet, nil
	default:
		return "", goerr.New("unknown format", goerr.V("input", s))
	}
}

// FromExt maps a file extension (with or without a leading dot, any case) to
// a Format. The boolean is false when the extension is not recognised.
func FromExt(ext string) (Format, bool) {
	ext = strings.ToLower(strings.TrimPrefix(ext, "."))
	switch ext {
	case "json":
		return FormatJSON, true
	case "yaml", "yml":
		return FormatYAML, true
	case "toml":
		return FormatTOML, true
	case "hcl", "tf":
		// .tf (Terraform) files are HCL syntactically; we accept it as an
		// alias for convenience.
		return FormatHCL, true
	case "jsonnet", "libsonnet":
		return FormatJsonnet, true
	default:
		return "", false
	}
}

// FromPath is a convenience for FromExt(filepath.Ext(path)).
func FromPath(path string) (Format, bool) {
	return FromExt(filepath.Ext(path))
}

// Heuristics used by Detect. Compiled once to avoid per-call allocation.
var (
	reTOMLSection = regexp.MustCompile(`^\[[A-Za-z_][A-Za-z0-9_.-]*\](?:\s*$|\s*#)`)
	reTOMLArrTbl  = regexp.MustCompile(`^\[\[[A-Za-z_][A-Za-z0-9_.-]*\]\]`)
	reHCLBlock    = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_-]*(\s+"[^"]*")+\s*\{`)
	reYAMLKey     = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_.-]*\s*:(\s|$)`)
	reAttrEquals  = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_-]*\s*=`)
)

// Detect makes a best-effort guess about the format of a byte buffer.
//
// Returns (format, true) when it can identify the input; otherwise (\"\", false).
// The caller is expected to surface a clear error and ask the user to supply
// --src when detection fails.
//
// Operates on the input byte slice without allocating a string copy of the
// whole buffer — useful when stdin or a large file is being sniffed.
func Detect(b []byte) (Format, bool) {
	trimmed := bytes.TrimSpace(b)
	if len(trimmed) == 0 {
		return "", false
	}

	// Jsonnet keywords that JSON cannot contain.
	if bytes.HasPrefix(trimmed, []byte("local ")) ||
		bytes.HasPrefix(trimmed, []byte("function(")) ||
		bytes.HasPrefix(trimmed, []byte("function ")) ||
		bytes.HasPrefix(trimmed, []byte("import ")) ||
		bytes.HasPrefix(trimmed, []byte("importstr ")) {
		return FormatJsonnet, true
	}

	// JSON-like start. `{` is unambiguously JSON-like (jsonnet may also start
	// with `{` but we prefer JSON; users can pass --src jsonnet explicitly).
	if trimmed[0] == '{' {
		return FormatJSON, true
	}
	// `[` is ambiguous: JSON array vs TOML section header. Decide by looking
	// at the first line.
	if trimmed[0] == '[' {
		firstLine := trimmed
		if idx := bytes.IndexByte(trimmed, '\n'); idx >= 0 {
			firstLine = trimmed[:idx]
		}
		firstLine = bytes.TrimRight(firstLine, " \t\r")
		if reTOMLArrTbl.Match(firstLine) || reTOMLSection.Match(firstLine) {
			return FormatTOML, true
		}
		return FormatJSON, true
	}

	// Line-oriented signals. bytes.Split returns sub-slices of the original
	// buffer (no copy per line).
	var (
		tomlSection int
		hclBlock    int
		yamlKey     int
		attrEquals  int
	)
	for _, line := range bytes.Split(b, []byte("\n")) {
		t := bytes.TrimSpace(line)
		if len(t) == 0 {
			continue
		}
		// Skip line-comment lines so they don't pollute the counters.
		if bytes.HasPrefix(t, []byte("#")) || bytes.HasPrefix(t, []byte("//")) {
			continue
		}
		switch {
		case reTOMLArrTbl.Match(t):
			tomlSection++
		case reTOMLSection.Match(t):
			tomlSection++
		case reHCLBlock.Match(t):
			hclBlock++
		case reYAMLKey.Match(t):
			yamlKey++
		case reAttrEquals.Match(t):
			attrEquals++
		}
	}

	switch {
	case tomlSection > 0:
		return FormatTOML, true
	case hclBlock > 0:
		return FormatHCL, true
	case yamlKey > 0:
		return FormatYAML, true
	case attrEquals > 0:
		// `key = value` is shared by HCL and TOML; with no section/block
		// signal we prefer HCL, since TOML normally has at least one
		// `[section]` in practical files.
		return FormatHCL, true
	default:
		return "", false
	}
}

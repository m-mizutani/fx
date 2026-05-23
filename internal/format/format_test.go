package format_test

import (
	"testing"

	"github.com/m-mizutani/fx/internal/format"
	"github.com/m-mizutani/gt"
)

func TestAll(t *testing.T) {
	all := format.All()
	gt.Equal(t, len(all), 5)
	// Spot-check ordering is stable.
	gt.Equal(t, all[0], format.FormatJSON)
	gt.Equal(t, all[4], format.FormatJsonnet)
}

func TestParse(t *testing.T) {
	cases := map[string]format.Format{
		"json":      format.FormatJSON,
		"JSON":      format.FormatJSON,
		"yaml":      format.FormatYAML,
		"yml":       format.FormatYAML,
		"  YAML  ":  format.FormatYAML,
		"toml":      format.FormatTOML,
		"hcl":       format.FormatHCL,
		"HCL":       format.FormatHCL,
		"jsonnet":   format.FormatJsonnet,
		"libsonnet": format.FormatJsonnet,
	}
	for in, want := range cases {
		got, err := format.Parse(in)
		gt.NoError(t, err)
		gt.Equal(t, got, want)
	}
}

func TestParseUnknown(t *testing.T) {
	_, err := format.Parse("xml")
	gt.Error(t, err)
}

func TestFromExt(t *testing.T) {
	cases := []struct {
		in   string
		want format.Format
		ok   bool
	}{
		{".json", format.FormatJSON, true},
		{"json", format.FormatJSON, true},
		{".JSON", format.FormatJSON, true},
		{".yaml", format.FormatYAML, true},
		{".yml", format.FormatYAML, true},
		{".toml", format.FormatTOML, true},
		{".hcl", format.FormatHCL, true},
		{".tf", format.FormatHCL, true},
		{".jsonnet", format.FormatJsonnet, true},
		{".libsonnet", format.FormatJsonnet, true},
		{".xml", "", false},
		{"", "", false},
	}
	for _, c := range cases {
		t.Run(c.in, func(t *testing.T) {
			got, ok := format.FromExt(c.in)
			gt.Equal(t, ok, c.ok)
			gt.Equal(t, got, c.want)
		})
	}
}

func TestFromPath(t *testing.T) {
	got, ok := format.FromPath("/tmp/foo.yaml")
	gt.True(t, ok)
	gt.Equal(t, got, format.FormatYAML)

	_, ok = format.FromPath("/tmp/foo")
	gt.False(t, ok)
}

func TestDetect(t *testing.T) {
	cases := []struct {
		name string
		in   string
		want format.Format
		ok   bool
	}{
		{"empty", "", "", false},
		{"whitespace", "   \n\t  ", "", false},
		{"json object", `{"a": 1}`, format.FormatJSON, true},
		{"json array", `[1, 2, 3]`, format.FormatJSON, true},
		{"json with leading whitespace", "   \n{\"a\":1}", format.FormatJSON, true},
		{"jsonnet local", `local x = 1; { v: x }`, format.FormatJsonnet, true},
		{"jsonnet function", `function(x) x + 1`, format.FormatJsonnet, true},
		{"jsonnet function paren", `function(x){ x: x }`, format.FormatJsonnet, true},
		{"jsonnet import", `import "lib.libsonnet"`, format.FormatJsonnet, true},
		{"jsonnet importstr", `importstr "lib"`, format.FormatJsonnet, true},
		{"yaml mapping", "foo: bar\nbaz: qux\n", format.FormatYAML, true},
		{"yaml with comments", "# hello\nfoo: bar\n", format.FormatYAML, true},
		{"toml section", "[server]\nport = 8080\n", format.FormatTOML, true},
		{"toml array of tables", "[[items]]\nname = \"a\"\n", format.FormatTOML, true},
		{"toml with comment first", "# config\n[server]\nport = 8080\n", format.FormatTOML, true},
		{"toml array tables with comment", "# items\n[[items]]\nname = \"a\"\n", format.FormatTOML, true},
		{"hcl block", `service "web" {
  port = 80
}`, format.FormatHCL, true},
		{"hcl bare attribute", "name = \"foo\"\n", format.FormatHCL, true},
		{"random bytes", "lkasjdlkasjdlkasjd", "", false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got, ok := format.Detect([]byte(c.in))
			gt.Equal(t, ok, c.ok)
			gt.Equal(t, got, c.want)
		})
	}
}

func TestDetectCommentsOnlyIgnored(t *testing.T) {
	// If a file is entirely comments / blank, detection should fail rather
	// than guess.
	_, ok := format.Detect([]byte("# only comment\n// another\n"))
	gt.False(t, ok)
}

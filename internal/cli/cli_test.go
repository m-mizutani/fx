package cli_test

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/m-mizutani/fx/internal/cli"
	"github.com/m-mizutani/gt"
)

// runCLI is a small helper invoking the CLI with the given args and IO buffers.
func runCLI(t *testing.T, args []string, stdin string) (int, string, string) {
	t.Helper()
	var stdout, stderr bytes.Buffer
	code := cli.Run(context.Background(), append([]string{"fx"}, args...),
		strings.NewReader(stdin), &stdout, &stderr)
	return code, stdout.String(), stderr.String()
}

// tempFile writes content into a file named name within t.TempDir().
func tempFile(t *testing.T, name, content string) string {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, name)
	gt.NoError(t, os.WriteFile(path, []byte(content), 0o600))
	return path
}

func TestRunStdinToStdoutExplicitFormats(t *testing.T) {
	code, stdout, stderr := runCLI(t,
		[]string{"--src", "json", "--dst", "yaml"},
		`{"a": 1, "b": "x"}`)
	gt.Equal(t, code, 0)
	gt.Equal(t, stderr, "")
	gt.S(t, stdout).Contains("a:")
	gt.S(t, stdout).Contains("b:")
}

func TestRunFileSrcAutoDetectFromExt(t *testing.T) {
	path := tempFile(t, "in.yaml", "k: v\n")
	code, stdout, stderr := runCLI(t,
		[]string{"--dst", "json", path}, "")
	if code != 0 {
		t.Fatalf("expected code 0, got %d, stderr=%s", code, stderr)
	}
	gt.S(t, stdout).Contains(`"k":`)
}

func TestRunDstAutoDetectFromOutExt(t *testing.T) {
	srcPath := tempFile(t, "in.json", `{"k": "v"}`)
	dstPath := filepath.Join(t.TempDir(), "out.yaml")
	code, _, stderr := runCLI(t,
		[]string{"--out", dstPath, srcPath}, "")
	if code != 0 {
		t.Fatalf("expected code 0, got %d, stderr=%s", code, stderr)
	}
	body, err := os.ReadFile(dstPath)
	gt.NoError(t, err)
	gt.S(t, string(body)).Contains("k:")
}

func TestRunStdinAutoDetectFromContent(t *testing.T) {
	// stdin with no --src, content sniffing decides.
	code, stdout, stderr := runCLI(t,
		[]string{"--dst", "yaml"},
		`{"a": 1}`)
	if code != 0 {
		t.Fatalf("expected code 0, got %d, stderr=%s", code, stderr)
	}
	gt.S(t, stdout).Contains("a:")
}

func TestRunStdinDetectionFails(t *testing.T) {
	code, _, stderr := runCLI(t,
		[]string{"--dst", "yaml"},
		"this is not anything")
	gt.NotEqual(t, code, 0)
	gt.S(t, stderr).Contains("detect")
}

func TestRunBothFormatsMissing(t *testing.T) {
	code, _, stderr := runCLI(t,
		[]string{},
		`{"a": 1}`)
	gt.NotEqual(t, code, 0)
	gt.S(t, stderr).Contains("destination format")
}

func TestRunInvalidSrcFlag(t *testing.T) {
	code, _, _ := runCLI(t, []string{"--src", "xml", "--dst", "yaml"}, `{}`)
	gt.NotEqual(t, code, 0)
}

func TestRunInvalidDstFlag(t *testing.T) {
	code, _, _ := runCLI(t, []string{"--src", "json", "--dst", "xml"}, `{}`)
	gt.NotEqual(t, code, 0)
}

func TestRunInputFileMissing(t *testing.T) {
	code, _, stderr := runCLI(t,
		[]string{"--src", "json", "--dst", "yaml", "/no/such/file.json"}, "")
	gt.NotEqual(t, code, 0)
	gt.S(t, stderr).Contains("open input")
}

func TestRunOutputDirInvalid(t *testing.T) {
	// Pointing --out at a non-existent directory should fail.
	code, _, stderr := runCLI(t,
		[]string{"--src", "json", "--dst", "yaml", "--out", "/no/such/dir/out.yaml"},
		`{"a":1}`)
	gt.NotEqual(t, code, 0)
	gt.S(t, stderr).Contains("create output")
}

func TestRunTooManyPositional(t *testing.T) {
	code, _, stderr := runCLI(t,
		[]string{"--src", "json", "--dst", "yaml", "a.json", "b.json"}, "")
	gt.NotEqual(t, code, 0)
	gt.S(t, stderr).Contains("positional")
}

func TestRunSrcFlagOverridesExtension(t *testing.T) {
	// File has .yaml extension but content is JSON; with --src json it works.
	path := tempFile(t, "trick.yaml", `{"a": 1}`)
	code, stdout, stderr := runCLI(t,
		[]string{"--src", "json", "--dst", "yaml", path}, "")
	if code != 0 {
		t.Fatalf("expected code 0, got %d, stderr=%s", code, stderr)
	}
	gt.S(t, stdout).Contains("a:")
}

func TestRunHelp(t *testing.T) {
	code, stdout, _ := runCLI(t, []string{"--help"}, "")
	gt.Equal(t, code, 0)
	gt.S(t, stdout).Contains("fx")
	// All supported format identifiers must be listed in --help.
	for _, name := range []string{"json", "yaml", "toml", "hcl", "jsonnet"} {
		gt.S(t, stdout).Contains(name)
	}
	// Short aliases should be visible too.
	gt.S(t, stdout).Contains("-s")
	gt.S(t, stdout).Contains("-d")
	gt.S(t, stdout).Contains("-o")
}

func TestRunShortFlags(t *testing.T) {
	// -s and -d should behave like --src and --dst.
	code, stdout, stderr := runCLI(t,
		[]string{"-s", "json", "-d", "yaml"},
		`{"k": "v"}`)
	if code != 0 {
		t.Fatalf("expected code 0, got %d, stderr=%s", code, stderr)
	}
	gt.S(t, stdout).Contains("k:")
}

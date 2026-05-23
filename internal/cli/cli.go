// Package cli builds the fx command-line interface using urfave/cli/v3.
// The Run function is the single entry point — main.go calls it and exits
// with its return code, and tests invoke it directly with custom IO so that
// no stdio interception is needed.
package cli

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"log/slog"
	"os"
	"strings"

	"github.com/m-mizutani/fx/internal/exchange"
	"github.com/m-mizutani/fx/internal/format"
	"github.com/m-mizutani/fx/internal/logging"
	"github.com/m-mizutani/goerr/v2"
	"github.com/urfave/cli/v3"
)

// supportedFormatList renders the list of supported format identifiers as
// they should be passed to --src / --dst. Derived from format.All so it stays
// in sync when a new format is added.
func supportedFormatList() string {
	names := make([]string, 0, len(format.All()))
	for _, f := range format.All() {
		names = append(names, string(f))
	}
	return strings.Join(names, " | ")
}

// Run executes the fx CLI with the given argv-style args. Stdin / stdout /
// stderr are injected so tests can capture them. The return value is an exit
// code suitable for os.Exit.
func Run(ctx context.Context, args []string, stdin io.Reader, stdout, stderr io.Writer) int {
	// Wire a context-scoped logger writing to stderr so library code can pick
	// it up without going through the global slog. Level is Info by default
	// (we expose no flag to change it yet).
	ctx = logging.With(ctx, logging.New(stderr, slog.LevelInfo))

	cmd := buildCommand(stdin, stdout)
	if err := cmd.Run(ctx, args); err != nil {
		_, _ = fmt.Fprintf(stderr, "fx: %s\n", err)
		return 1
	}
	return 0
}

// buildCommand wires the cli.Command. stdin / stdout are closed over so the
// Action can read/write them without depending on os globals.
func buildCommand(stdin io.Reader, stdout io.Writer) *cli.Command {
	var (
		srcFlag string
		dstFlag string
		outFlag string
	)
	formats := supportedFormatList()
	return &cli.Command{
		Name:      "fx",
		Usage:     "Format Exchange CLI: convert between json/yaml/toml/hcl/jsonnet.",
		ArgsUsage: "[INPUT]",
		// Route help/usage to our injected stdout so tests can capture it.
		Writer: stdout,
		Description: "Read INPUT (or stdin) in --src format and write it to --out (or stdout)\n" +
			"in --dst format. Either flag may be omitted: --src is auto-detected from\n" +
			"the file extension or content; --dst is inferred from --out's extension.\n\n" +
			"Supported formats: " + formats + "\n" +
			"  - jsonnet is decode-only (cannot be used as --dst).\n" +
			"  - yaml accepts \"yml\" as an alias; jsonnet accepts \"libsonnet\".",
		Flags: []cli.Flag{
			&cli.StringFlag{
				Name:        "src",
				Aliases:     []string{"s"},
				Usage:       "source format [" + formats + "] (auto-detect when omitted)",
				Destination: &srcFlag,
			},
			&cli.StringFlag{
				Name:        "dst",
				Aliases:     []string{"d"},
				Usage:       "destination format [" + formats + "] (inferred from --out when omitted)",
				Destination: &dstFlag,
			},
			&cli.StringFlag{
				Name:        "out",
				Aliases:     []string{"o"},
				Usage:       "output file path (default: stdout)",
				Destination: &outFlag,
			},
		},
		Action: func(ctx context.Context, cmd *cli.Command) error {
			return runConvert(cmd, stdin, stdout, srcFlag, dstFlag, outFlag)
		},
	}
}

// runConvert is the actual conversion driver: it resolves IO and formats,
// then delegates to exchange.Exchange.
func runConvert(cmd *cli.Command, stdin io.Reader, stdout io.Writer,
	srcFlag, dstFlag, outFlag string,
) error {
	positional := cmd.Args().Slice()
	if len(positional) > 1 {
		return goerr.New("at most one positional argument is allowed",
			goerr.V("got", len(positional)))
	}
	inputPath := ""
	if len(positional) == 1 {
		inputPath = positional[0]
	}

	// Resolve input reader. Files are closed at function exit; stdin we leave
	// alone (the caller owns it).
	in, closeIn, err := openInput(inputPath, stdin)
	if err != nil {
		return err
	}
	defer closeIn()

	// Resolve output writer.
	out, closeOut, err := openOutput(outFlag, stdout)
	if err != nil {
		return err
	}
	defer closeOut()

	// Resolve src.
	src, in, err := resolveSrc(srcFlag, inputPath, in)
	if err != nil {
		return err
	}

	// Resolve dst.
	dst, err := resolveDst(dstFlag, outFlag)
	if err != nil {
		return err
	}

	return exchange.Exchange(src, dst, in, out)
}

// openInput returns a reader for either the given file path or, if empty,
// the provided stdin. The returned closer is always non-nil and safe to call.
func openInput(path string, stdin io.Reader) (io.Reader, func(), error) {
	if path == "" {
		return stdin, func() {}, nil
	}
	f, err := os.Open(path)
	if err != nil {
		return nil, func() {}, goerr.Wrap(err, "open input", goerr.V("path", path))
	}
	return f, func() { _ = f.Close() }, nil
}

// openOutput returns a writer for either the given file path or, if empty,
// the provided stdout. The closer is no-op for stdout.
func openOutput(path string, stdout io.Writer) (io.Writer, func(), error) {
	if path == "" {
		return stdout, func() {}, nil
	}
	f, err := os.Create(path)
	if err != nil {
		return nil, func() {}, goerr.Wrap(err, "create output", goerr.V("path", path))
	}
	return f, func() { _ = f.Close() }, nil
}

// resolveSrc figures out the source format from (in priority order)
// the --src flag, the input file extension, or the bytes of the input.
// When content sniffing is required, the input bytes are buffered and a new
// reader is returned so the buffered bytes are not lost.
func resolveSrc(srcFlag, inputPath string, in io.Reader) (format.Format, io.Reader, error) {
	if srcFlag != "" {
		f, err := format.Parse(srcFlag)
		if err != nil {
			return "", in, goerr.Wrap(err, "parse --src")
		}
		return f, in, nil
	}
	if inputPath != "" {
		if f, ok := format.FromPath(inputPath); ok {
			return f, in, nil
		}
	}
	// Content sniffing — slurp the input fully (acceptable for CLI use).
	data, err := io.ReadAll(in)
	if err != nil {
		return "", in, goerr.Wrap(err, "read input for format detection")
	}
	f, ok := format.Detect(data)
	if !ok {
		return "", in, goerr.New("could not detect source format; pass --src",
			goerr.V("bytes_read", len(data)))
	}
	return f, bytes.NewReader(data), nil
}

// resolveDst figures out the destination format from the --dst flag or
// from the --out file extension.
func resolveDst(dstFlag, outFlag string) (format.Format, error) {
	if dstFlag != "" {
		f, err := format.Parse(dstFlag)
		if err != nil {
			return "", goerr.Wrap(err, "parse --dst")
		}
		return f, nil
	}
	if outFlag != "" {
		if f, ok := format.FromPath(outFlag); ok {
			return f, nil
		}
	}
	return "", goerr.New("could not determine destination format; pass --dst or --out with a known extension")
}

package main

import (
	"flag"
	"fmt"
	"io"
	"os"

	"github.com/umars28/envdiff/internal/diff"
	"github.com/umars28/envdiff/internal/envfile"
	"github.com/umars28/envdiff/internal/mask"
	"github.com/umars28/envdiff/internal/report"
)

const version = "0.1.0"

const (
	exitSame = 0
	exitDiff = 1
	exitFail = 2
)

func main() {
	os.Exit(run(os.Args[1:], os.Stdout, os.Stderr))
}

func run(args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("envdiff", flag.ContinueOnError)
	fs.SetOutput(stderr)
	fs.Usage = func() {
		fmt.Fprintf(stderr, "usage: envdiff [-q] [-version] <old.env> <new.env>\n")
		fs.PrintDefaults()
	}
	quiet := fs.Bool("q", false, "suppress stdout and report via exit code only")
	showVersion := fs.Bool("version", false, "print the version and exit")

	if err := fs.Parse(args); err != nil {
		return exitFail
	}
	if *showVersion {
		fmt.Fprintf(stdout, "envdiff %s\n", version)
		return exitSame
	}
	if fs.NArg() != 2 {
		fs.Usage()
		return exitFail
	}

	left, err := envfile.ParseFile(fs.Arg(0))
	if err != nil {
		fmt.Fprintf(stderr, "envdiff: %v\n", err)
		return exitFail
	}
	right, err := envfile.ParseFile(fs.Arg(1))
	if err != nil {
		fmt.Fprintf(stderr, "envdiff: %v\n", err)
		return exitFail
	}

	if err := report.Warnings(stderr, left, right); err != nil {
		fmt.Fprintf(stderr, "envdiff: %v\n", err)
		return exitFail
	}

	m, err := mask.New()
	if err != nil {
		fmt.Fprintf(stderr, "envdiff: %v\n", err)
		return exitFail
	}

	result := diff.Compare(left, right, m)
	if !*quiet {
		if err := report.Text(stdout, result); err != nil {
			fmt.Fprintf(stderr, "envdiff: %v\n", err)
			return exitFail
		}
	}
	if result.HasChanges() {
		return exitDiff
	}
	return exitSame
}

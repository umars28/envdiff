package main

import (
	"bytes"
	"path/filepath"
	"strings"
	"testing"
)

const sentinel = "hunter2-TOTALLY-SECRET"

var fixtures = []string{"base.env", "same.env", "changed.env", "dupes.env", "malformed.env"}

func fixture(name string) string { return filepath.Join("testdata", name) }

func TestRunExitCodes(t *testing.T) {
	tests := []struct {
		name string
		args []string
		want int
	}{
		{"identical files", []string{fixture("base.env"), fixture("same.env")}, 0},
		{"differing files", []string{fixture("base.env"), fixture("changed.env")}, 1},
		{"missing left file", []string{fixture("nope.env"), fixture("base.env")}, 2},
		{"missing right file", []string{fixture("base.env"), fixture("nope.env")}, 2},
		{"malformed file", []string{fixture("base.env"), fixture("malformed.env")}, 2},
		{"no arguments", nil, 2},
		{"one argument", []string{fixture("base.env")}, 2},
		{"three arguments", []string{fixture("base.env"), fixture("same.env"), fixture("changed.env")}, 2},
		{"unknown flag", []string{"-nope", fixture("base.env"), fixture("same.env")}, 2},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var stdout, stderr bytes.Buffer
			if got := run(tt.args, &stdout, &stderr); got != tt.want {
				t.Errorf("run(%v) = %d, want %d (stderr: %s)", tt.args, got, tt.want, stderr.String())
			}
		})
	}
}

func TestRunWritesDiff(t *testing.T) {
	var stdout, stderr bytes.Buffer
	if code := run([]string{fixture("base.env"), fixture("changed.env")}, &stdout, &stderr); code != 1 {
		t.Fatalf("run() = %d, want 1", code)
	}
	out := stdout.String()
	for _, want := range []string{
		"--- " + fixture("base.env"),
		"+++ " + fixture("changed.env"),
		"+ NEW_KEY",
		"- API_KEY",
		"~ DATABASE_URL",
		"3 changed, 1 unchanged",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("stdout missing %q; got:\n%s", want, out)
		}
	}
	if stderr.Len() != 0 {
		t.Errorf("stderr = %q, want empty", stderr.String())
	}
}

func TestRunQuiet(t *testing.T) {
	tests := []struct {
		name string
		args []string
		want int
	}{
		{"identical", []string{"-q", fixture("base.env"), fixture("same.env")}, 0},
		{"differing", []string{"-q", fixture("base.env"), fixture("changed.env")}, 1},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var stdout, stderr bytes.Buffer
			if got := run(tt.args, &stdout, &stderr); got != tt.want {
				t.Errorf("run(%v) = %d, want %d", tt.args, got, tt.want)
			}
			if stdout.Len() != 0 {
				t.Errorf("stdout = %q, want empty under -q", stdout.String())
			}
		})
	}
}

func TestRunVersion(t *testing.T) {
	var stdout, stderr bytes.Buffer
	if got := run([]string{"-version"}, &stdout, &stderr); got != 0 {
		t.Errorf("run(-version) = %d, want 0", got)
	}
	if !strings.HasPrefix(stdout.String(), "envdiff ") {
		t.Errorf("stdout = %q, want it to start with %q", stdout.String(), "envdiff ")
	}
}

func TestRunWarnsOnDuplicates(t *testing.T) {
	var stdout, stderr bytes.Buffer
	run([]string{fixture("base.env"), fixture("dupes.env")}, &stdout, &stderr)
	want := "warning: " + fixture("dupes.env") + ":3: duplicate key DATABASE_URL (first seen line 1)\n"
	if stderr.String() != want {
		t.Errorf("stderr = %q, want %q", stderr.String(), want)
	}
}

func TestRunNeverLeaksValues(t *testing.T) {
	var args [][]string
	for _, left := range fixtures {
		for _, right := range fixtures {
			args = append(args,
				[]string{fixture(left), fixture(right)},
				[]string{"-q", fixture(left), fixture(right)},
			)
		}
		args = append(args,
			[]string{fixture(left), fixture("nope.env")},
			[]string{fixture("nope.env"), fixture(left)},
		)
	}

	for _, a := range args {
		var stdout, stderr bytes.Buffer
		run(a, &stdout, &stderr)
		if strings.Contains(stdout.String(), sentinel) {
			t.Errorf("run(%v) leaked the sentinel on stdout:\n%s", a, stdout.String())
		}
		if strings.Contains(stderr.String(), sentinel) {
			t.Errorf("run(%v) leaked the sentinel on stderr:\n%s", a, stderr.String())
		}
	}
}

func TestRunSaltVariesPerRun(t *testing.T) {
	first, second := &bytes.Buffer{}, &bytes.Buffer{}
	var stderr bytes.Buffer
	run([]string{fixture("base.env"), fixture("changed.env")}, first, &stderr)
	run([]string{fixture("base.env"), fixture("changed.env")}, second, &stderr)
	if first.String() == second.String() {
		t.Error("two runs produced identical output; the salt is not per-run")
	}
}

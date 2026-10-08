package report

import (
	"bytes"
	"errors"
	"testing"

	"github.com/umars28/envdiff/internal/diff"
	"github.com/umars28/envdiff/internal/envfile"
	"github.com/umars28/envdiff/internal/mask"
)

type failingWriter struct{}

var errWrite = errors.New("write failed")

func (failingWriter) Write([]byte) (int, error) { return 0, errWrite }

func TestText(t *testing.T) {
	r := &diff.Result{
		LeftPath:  ".env.example",
		RightPath: ".env.production",
		Entries: []diff.Entry{
			{Key: "NEW_RELIC_KEY", Kind: diff.Added, New: mask.Print("<masked:3f9a1c>")},
			{Key: "LEGACY_SMTP_HOST", Kind: diff.Removed, Old: mask.Print("<masked:7b02de>")},
			{Key: "DATABASE_URL", Kind: diff.Changed, Old: mask.Print("<masked:1c4e88>"), New: mask.Print("<masked:a90f21>")},
		},
		SameCount: 12,
	}
	want := "--- .env.example\n" +
		"+++ .env.production\n" +
		"\n" +
		"+ NEW_RELIC_KEY        <masked:3f9a1c>\n" +
		"- LEGACY_SMTP_HOST     <masked:7b02de>\n" +
		"~ DATABASE_URL         <masked:1c4e88> -> <masked:a90f21>\n" +
		"\n" +
		"3 changed, 12 unchanged\n" +
		"fingerprints are salted per run; comparable only within this output\n"

	var buf bytes.Buffer
	if err := Text(&buf, r); err != nil {
		t.Fatalf("Text() error = %v", err)
	}
	if buf.String() != want {
		t.Errorf("Text() =\n%q\nwant\n%q", buf.String(), want)
	}
}

func TestTextLongKeyWidensColumn(t *testing.T) {
	r := &diff.Result{
		LeftPath:  "a.env",
		RightPath: "b.env",
		Entries: []diff.Entry{
			{Key: "A_VERY_LONG_KEY_NAME_INDEED", Kind: diff.Added, New: mask.Print("<masked:000001>")},
			{Key: "SHORT", Kind: diff.Added, New: mask.Print("<masked:000002>")},
		},
	}
	want := "--- a.env\n" +
		"+++ b.env\n" +
		"\n" +
		"+ A_VERY_LONG_KEY_NAME_INDEED <masked:000001>\n" +
		"+ SHORT                       <masked:000002>\n" +
		"\n" +
		"2 changed, 0 unchanged\n" +
		"fingerprints are salted per run; comparable only within this output\n"

	var buf bytes.Buffer
	if err := Text(&buf, r); err != nil {
		t.Fatalf("Text() error = %v", err)
	}
	if buf.String() != want {
		t.Errorf("Text() =\n%q\nwant\n%q", buf.String(), want)
	}
}

func TestTextEmpty(t *testing.T) {
	r := &diff.Result{LeftPath: "a.env", RightPath: "b.env"}
	want := "--- a.env\n" +
		"+++ b.env\n" +
		"\n" +
		"0 changed, 0 unchanged\n" +
		"fingerprints are salted per run; comparable only within this output\n"

	var buf bytes.Buffer
	if err := Text(&buf, r); err != nil {
		t.Fatalf("Text() error = %v", err)
	}
	if buf.String() != want {
		t.Errorf("Text() =\n%q\nwant\n%q", buf.String(), want)
	}
}

func TestTextWriteError(t *testing.T) {
	if err := Text(failingWriter{}, &diff.Result{}); !errors.Is(err, errWrite) {
		t.Errorf("Text() error = %v, want %v", err, errWrite)
	}
}

func TestWarnings(t *testing.T) {
	f := &envfile.File{
		Path: ".env.production",
		Duplicates: []envfile.Duplicate{
			{Key: "DATABASE_URL", FirstLine: 4, SecondLine: 19},
			{Key: "PORT", FirstLine: 7, SecondLine: 23},
		},
	}
	want := "warning: .env.production:19: duplicate key DATABASE_URL (first seen line 4)\n" +
		"warning: .env.production:23: duplicate key PORT (first seen line 7)\n"

	var buf bytes.Buffer
	if err := Warnings(&buf, f); err != nil {
		t.Fatalf("Warnings() error = %v", err)
	}
	if buf.String() != want {
		t.Errorf("Warnings() =\n%q\nwant\n%q", buf.String(), want)
	}
}

func TestWarningsMultipleFiles(t *testing.T) {
	left := &envfile.File{Path: "a.env", Duplicates: []envfile.Duplicate{{Key: "A", FirstLine: 1, SecondLine: 2}}}
	right := &envfile.File{Path: "b.env", Duplicates: []envfile.Duplicate{{Key: "B", FirstLine: 3, SecondLine: 4}}}
	want := "warning: a.env:2: duplicate key A (first seen line 1)\n" +
		"warning: b.env:4: duplicate key B (first seen line 3)\n"

	var buf bytes.Buffer
	if err := Warnings(&buf, left, right); err != nil {
		t.Fatalf("Warnings() error = %v", err)
	}
	if buf.String() != want {
		t.Errorf("Warnings() =\n%q\nwant\n%q", buf.String(), want)
	}
}

func TestWarningsEmpty(t *testing.T) {
	var buf bytes.Buffer
	if err := Warnings(&buf, &envfile.File{Path: "a.env"}); err != nil {
		t.Fatalf("Warnings() error = %v", err)
	}
	if buf.Len() != 0 {
		t.Errorf("Warnings() = %q, want no output", buf.String())
	}
	if err := Warnings(&buf); err != nil {
		t.Fatalf("Warnings() with no files error = %v", err)
	}
}

func TestWarningsWriteError(t *testing.T) {
	f := &envfile.File{Path: "a.env", Duplicates: []envfile.Duplicate{{Key: "A", FirstLine: 1, SecondLine: 2}}}
	if err := Warnings(failingWriter{}, f); !errors.Is(err, errWrite) {
		t.Errorf("Warnings() error = %v, want %v", err, errWrite)
	}
}

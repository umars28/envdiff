package report

import (
	"fmt"
	"io"
	"strings"

	"github.com/umars28/envdiff/internal/diff"
	"github.com/umars28/envdiff/internal/envfile"
)

const (
	minKeyWidth = 20
	caveat      = "fingerprints are salted per run; comparable only within this output"
)

func Text(w io.Writer, r *diff.Result) error {
	var b strings.Builder
	fmt.Fprintf(&b, "--- %s\n+++ %s\n\n", r.LeftPath, r.RightPath)

	if len(r.Entries) > 0 {
		width := minKeyWidth
		for _, e := range r.Entries {
			if len(e.Key) > width {
				width = len(e.Key)
			}
		}
		for _, e := range r.Entries {
			fmt.Fprintf(&b, "%s %-*s %s", e.Kind.Sigil(), width, e.Key, prints(e))
			b.WriteString("\n")
		}
		b.WriteString("\n")
	}

	fmt.Fprintf(&b, "%d changed, %d unchanged\n%s\n", len(r.Entries), r.SameCount, caveat)

	_, err := io.WriteString(w, b.String())
	return err
}

func Warnings(w io.Writer, files ...*envfile.File) error {
	var b strings.Builder
	for _, f := range files {
		for _, d := range f.Duplicates {
			fmt.Fprintf(&b, "warning: %s:%d: duplicate key %s (first seen line %d)\n",
				f.Path, d.SecondLine, d.Key, d.FirstLine)
		}
	}
	if b.Len() == 0 {
		return nil
	}
	_, err := io.WriteString(w, b.String())
	return err
}

func prints(e diff.Entry) string {
	switch e.Kind {
	case diff.Added:
		return e.New.String()
	case diff.Removed:
		return e.Old.String()
	default:
		return e.Old.String() + " -> " + e.New.String()
	}
}

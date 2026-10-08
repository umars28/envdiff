package diff

import (
	"reflect"
	"testing"

	"github.com/umars28/envdiff/internal/envfile"
	"github.com/umars28/envdiff/internal/mask"
)

func file(path string, pairs ...string) *envfile.File {
	f := &envfile.File{Path: path}
	for i := 0; i < len(pairs); i += 2 {
		f.Entries = append(f.Entries, envfile.Entry{Key: pairs[i], Value: pairs[i+1], Line: i/2 + 1})
	}
	return f
}

func masker() *mask.Masker { return mask.NewWithSalt([]byte("fixed-salt")) }

func TestCompareClassification(t *testing.T) {
	m := masker()
	left := file("old.env", "SAME", "s", "REMOVED", "r", "CHANGED", "before")
	right := file("new.env", "SAME", "s", "ADDED", "a", "CHANGED", "after")

	got := Compare(left, right, m)

	if got.LeftPath != "old.env" || got.RightPath != "new.env" {
		t.Errorf("paths = %q, %q; want old.env, new.env", got.LeftPath, got.RightPath)
	}
	if got.SameCount != 1 {
		t.Errorf("SameCount = %d, want 1", got.SameCount)
	}
	want := []Entry{
		{Key: "ADDED", Kind: Added, New: m.Fingerprint("a")},
		{Key: "CHANGED", Kind: Changed, Old: m.Fingerprint("before"), New: m.Fingerprint("after")},
		{Key: "REMOVED", Kind: Removed, Old: m.Fingerprint("r")},
	}
	if !reflect.DeepEqual(got.Entries, want) {
		t.Errorf("Entries = %+v, want %+v", got.Entries, want)
	}
}

func TestCompareSortsLexicographically(t *testing.T) {
	left := file("old.env")
	right := file("new.env", "ZED", "1", "alpha", "1", "BETA", "1", "ALPHA", "1")

	got := Compare(left, right, masker())

	var keys []string
	for _, e := range got.Entries {
		keys = append(keys, e.Key)
	}
	want := []string{"ALPHA", "BETA", "ZED", "alpha"}
	if !reflect.DeepEqual(keys, want) {
		t.Errorf("keys = %v, want %v", keys, want)
	}
}

func TestCompareSharedValueSharesPrint(t *testing.T) {
	left := file("old.env")
	right := file("new.env", "A", "same", "B", "same", "C", "other")

	got := Compare(left, right, masker())

	if got.Entries[0].New != got.Entries[1].New {
		t.Errorf("A and B share a value but fingerprints differ: %q vs %q", got.Entries[0].New, got.Entries[1].New)
	}
	if got.Entries[0].New == got.Entries[2].New {
		t.Error("A and C differ in value but share a fingerprint")
	}
}

func TestCompareEdges(t *testing.T) {
	m := masker()

	empty := Compare(file("old.env"), file("new.env"), m)
	if len(empty.Entries) != 0 || empty.SameCount != 0 {
		t.Errorf("empty vs empty = %+v, want no entries and no sames", empty)
	}
	if empty.HasChanges() {
		t.Error("HasChanges() = true for an empty result")
	}

	populated := Compare(file("old.env"), file("new.env", "A", "1"), m)
	if len(populated.Entries) != 1 || populated.Entries[0].Kind != Added {
		t.Errorf("empty vs populated = %+v, want one Added entry", populated.Entries)
	}
	if !populated.HasChanges() {
		t.Error("HasChanges() = false with one entry")
	}

	sameOnly := Compare(file("old.env", "A", "1"), file("new.env", "A", "1"), m)
	if sameOnly.HasChanges() || sameOnly.SameCount != 1 {
		t.Errorf("identical files = %+v, want no changes and SameCount 1", sameOnly)
	}
}

func TestCompareEmptyValues(t *testing.T) {
	m := masker()
	got := Compare(file("old.env", "A", ""), file("new.env", "A", "1"), m)
	want := []Entry{{Key: "A", Kind: Changed, Old: m.Fingerprint(""), New: m.Fingerprint("1")}}
	if !reflect.DeepEqual(got.Entries, want) {
		t.Errorf("Entries = %+v, want %+v", got.Entries, want)
	}
}

func TestKindSigil(t *testing.T) {
	tests := map[Kind]string{Added: "+", Removed: "-", Changed: "~"}
	for kind, want := range tests {
		if got := kind.Sigil(); got != want {
			t.Errorf("Kind(%d).Sigil() = %q, want %q", kind, got, want)
		}
	}
}

package diff

import (
	"sort"

	"github.com/umars28/envdiff/internal/envfile"
	"github.com/umars28/envdiff/internal/mask"
)

type Kind int

const (
	Added Kind = iota
	Removed
	Changed
)

func (k Kind) Sigil() string {
	switch k {
	case Added:
		return "+"
	case Removed:
		return "-"
	default:
		return "~"
	}
}

type Entry struct {
	Key      string
	Kind     Kind
	Old, New mask.Print
}

type Result struct {
	LeftPath, RightPath string
	Entries             []Entry
	SameCount           int
}

func (r *Result) HasChanges() bool {
	return len(r.Entries) > 0
}

func Compare(left, right *envfile.File, m *mask.Masker) *Result {
	r := &Result{LeftPath: left.Path, RightPath: right.Path}

	seen := make(map[string]bool)
	var keys []string
	for _, key := range append(left.Keys(), right.Keys()...) {
		if seen[key] {
			continue
		}
		seen[key] = true
		keys = append(keys, key)
	}
	sort.Strings(keys)

	for _, key := range keys {
		oldValue, inLeft := left.Lookup(key)
		newValue, inRight := right.Lookup(key)
		switch {
		case !inLeft:
			r.Entries = append(r.Entries, Entry{Key: key, Kind: Added, New: m.Fingerprint(newValue)})
		case !inRight:
			r.Entries = append(r.Entries, Entry{Key: key, Kind: Removed, Old: m.Fingerprint(oldValue)})
		case oldValue != newValue:
			r.Entries = append(r.Entries, Entry{
				Key:  key,
				Kind: Changed,
				Old:  m.Fingerprint(oldValue),
				New:  m.Fingerprint(newValue),
			})
		default:
			r.SameCount++
		}
	}
	return r
}

package envfile

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"strings"
)

const bom = "\xef\xbb\xbf"

type Entry struct {
	Key, Value string
	Line       int
}

type Duplicate struct {
	Key                   string
	FirstLine, SecondLine int
}

type File struct {
	Path       string
	Entries    []Entry
	Duplicates []Duplicate
}

type ParseError struct {
	Path   string
	Line   int
	Reason string
}

func (e *ParseError) Error() string {
	return fmt.Sprintf("%s:%d: %s", e.Path, e.Line, e.Reason)
}

func Parse(path string, r io.Reader) (*File, error) {
	f := &File{Path: path}
	index := make(map[string]int)

	sc := bufio.NewScanner(r)
	for lineNo := 1; sc.Scan(); lineNo++ {
		line := sc.Text()
		if lineNo == 1 {
			line = strings.TrimPrefix(line, bom)
		}
		line = strings.TrimSuffix(line, "\r")
		line = strings.TrimLeft(line, " \t")
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		key, value, reason := split(line)
		if reason != "" {
			return nil, &ParseError{Path: path, Line: lineNo, Reason: reason}
		}

		if at, ok := index[key]; ok {
			f.Duplicates = append(f.Duplicates, Duplicate{
				Key:        key,
				FirstLine:  f.Entries[at].Line,
				SecondLine: lineNo,
			})
			f.Entries[at].Value = value
			f.Entries[at].Line = lineNo
			continue
		}
		index[key] = len(f.Entries)
		f.Entries = append(f.Entries, Entry{Key: key, Value: value, Line: lineNo})
	}
	if err := sc.Err(); err != nil {
		return nil, err
	}
	return f, nil
}

func ParseFile(path string) (*File, error) {
	r, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer r.Close()
	return Parse(path, r)
}

func (f *File) Lookup(key string) (string, bool) {
	for _, e := range f.Entries {
		if e.Key == key {
			return e.Value, true
		}
	}
	return "", false
}

func (f *File) Keys() []string {
	keys := make([]string, 0, len(f.Entries))
	for _, e := range f.Entries {
		keys = append(keys, e.Key)
	}
	return keys
}

func split(line string) (key, value, reason string) {
	line = strings.TrimPrefix(line, "export ")
	line = strings.TrimLeft(line, " \t")

	eq := strings.IndexByte(line, '=')
	if eq < 0 {
		return "", "", "line is not a KEY=value assignment"
	}
	key = strings.TrimRight(line[:eq], " \t")
	if key == "" {
		return "", "", "assignment has an empty key"
	}
	if strings.ContainsAny(key, " \t") {
		return "", "", "key contains whitespace"
	}
	return key, unquote(line[eq+1:]), ""
}

func unquote(value string) string {
	if len(value) < 2 {
		return value
	}
	q := value[0]
	if (q == '"' || q == '\'') && value[len(value)-1] == q {
		return value[1 : len(value)-1]
	}
	return value
}

package envfile

import (
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func TestParseEntries(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  []Entry
	}{
		{
			name:  "simple assignment",
			input: "A=1\n",
			want:  []Entry{{Key: "A", Value: "1", Line: 1}},
		},
		{
			name:  "export prefix is stripped",
			input: "export A=1\n",
			want:  []Entry{{Key: "A", Value: "1", Line: 1}},
		},
		{
			name:  "double quotes stripped",
			input: `A="one two"` + "\n",
			want:  []Entry{{Key: "A", Value: "one two", Line: 1}},
		},
		{
			name:  "single quotes stripped",
			input: "A='one two'\n",
			want:  []Entry{{Key: "A", Value: "one two", Line: 1}},
		},
		{
			name:  "mismatched quotes are value data",
			input: `A="one'` + "\n",
			want:  []Entry{{Key: "A", Value: `"one'`, Line: 1}},
		},
		{
			name:  "quoted remainder is opaque",
			input: `A="a\nb${VAR}"` + "\n",
			want:  []Entry{{Key: "A", Value: `a\nb${VAR}`, Line: 1}},
		},
		{
			name:  "inline hash is value data",
			input: "PASSWORD=abc#123\n",
			want:  []Entry{{Key: "PASSWORD", Value: "abc#123", Line: 1}},
		},
		{
			name:  "comment at line start skipped",
			input: "# PASSWORD=secret\nA=1\n",
			want:  []Entry{{Key: "A", Value: "1", Line: 2}},
		},
		{
			name:  "comment after leading whitespace skipped",
			input: "   \t# nope\nA=1\n",
			want:  []Entry{{Key: "A", Value: "1", Line: 2}},
		},
		{
			name:  "blank lines skipped",
			input: "\n\nA=1\n   \n\nB=2\n",
			want:  []Entry{{Key: "A", Value: "1", Line: 3}, {Key: "B", Value: "2", Line: 6}},
		},
		{
			name:  "empty value",
			input: "A=\n",
			want:  []Entry{{Key: "A", Value: "", Line: 1}},
		},
		{
			name:  "value keeps later equals signs",
			input: "A=b=c\n",
			want:  []Entry{{Key: "A", Value: "b=c", Line: 1}},
		},
		{
			name:  "whitespace around key trimmed",
			input: "  A =1\n",
			want:  []Entry{{Key: "A", Value: "1", Line: 1}},
		},
		{
			name:  "no trailing newline",
			input: "A=1",
			want:  []Entry{{Key: "A", Value: "1", Line: 1}},
		},
		{
			name:  "trailing carriage return stripped",
			input: "A=1\r\n",
			want:  []Entry{{Key: "A", Value: "1", Line: 1}},
		},
		{
			name:  "leading BOM stripped",
			input: "\xef\xbb\xbfA=1\n",
			want:  []Entry{{Key: "A", Value: "1", Line: 1}},
		},
		{
			name:  "duplicate key last wins",
			input: "A=1\nA=2\n",
			want:  []Entry{{Key: "A", Value: "2", Line: 2}},
		},
		{
			name:  "empty input",
			input: "",
			want:  nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			f, err := Parse("in.env", strings.NewReader(tt.input))
			if err != nil {
				t.Fatalf("Parse() error = %v", err)
			}
			if !reflect.DeepEqual(f.Entries, tt.want) {
				t.Errorf("Entries = %+v, want %+v", f.Entries, tt.want)
			}
			if f.Path != "in.env" {
				t.Errorf("Path = %q, want %q", f.Path, "in.env")
			}
		})
	}
}

func TestParseMalformed(t *testing.T) {
	tests := []struct {
		name string
		line string
	}{
		{"no separator", "this line is junk"},
		{"empty key", "=value"},
		{"whitespace inside key", "foo bar=baz"},
		{"export with no assignment", "export FOO"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := Parse("secrets.env", strings.NewReader("A=1\n"+tt.line+"\n"))
			if err == nil {
				t.Fatal("Parse() error = nil, want *ParseError")
			}
			perr, ok := err.(*ParseError)
			if !ok {
				t.Fatalf("Parse() error type = %T, want *ParseError", err)
			}
			if perr.Path != "secrets.env" || perr.Line != 2 {
				t.Errorf("ParseError = %+v, want Path secrets.env Line 2", perr)
			}
			msg := perr.Error()
			for _, want := range []string{"secrets.env", "2", perr.Reason} {
				if !strings.Contains(msg, want) {
					t.Errorf("Error() = %q, want it to contain %q", msg, want)
				}
			}
			if strings.Contains(msg, tt.line) {
				t.Errorf("Error() = %q, must not contain the offending line", msg)
			}
		})
	}
}

func TestParseErrorOmitsValues(t *testing.T) {
	const sentinel = "hunter2-TOTALLY-SECRET"
	_, err := Parse("in.env", strings.NewReader("A="+sentinel+"\njunk "+sentinel+"\n"))
	if err == nil {
		t.Fatal("Parse() error = nil, want *ParseError")
	}
	if strings.Contains(err.Error(), sentinel) {
		t.Errorf("Error() = %q, leaked the sentinel", err.Error())
	}
}

func TestLookup(t *testing.T) {
	f, err := Parse("in.env", strings.NewReader("A=1\nB=2\nA=3\n"))
	if err != nil {
		t.Fatalf("Parse() error = %v", err)
	}
	if got, ok := f.Lookup("A"); !ok || got != "3" {
		t.Errorf("Lookup(A) = %q, %v; want %q, true", got, ok, "3")
	}
	if got, ok := f.Lookup("MISSING"); ok || got != "" {
		t.Errorf("Lookup(MISSING) = %q, %v; want %q, false", got, ok, "")
	}
}

func TestKeys(t *testing.T) {
	f, err := Parse("in.env", strings.NewReader("B=2\nA=1\nB=3\n"))
	if err != nil {
		t.Fatalf("Parse() error = %v", err)
	}
	want := []string{"B", "A"}
	if !reflect.DeepEqual(f.Keys(), want) {
		t.Errorf("Keys() = %v, want %v", f.Keys(), want)
	}
}

func TestDuplicates(t *testing.T) {
	f, err := ParseFile(filepath.Join("testdata", "duplicate.env"))
	if err != nil {
		t.Fatalf("ParseFile() error = %v", err)
	}
	want := []Duplicate{{Key: "DATABASE_URL", FirstLine: 2, SecondLine: 4}}
	if !reflect.DeepEqual(f.Duplicates, want) {
		t.Errorf("Duplicates = %+v, want %+v", f.Duplicates, want)
	}
	if got, ok := f.Lookup("DATABASE_URL"); !ok || got != "two" {
		t.Errorf("Lookup(DATABASE_URL) = %q, %v; want %q, true", got, ok, "two")
	}
}

func TestParseFileFixtures(t *testing.T) {
	tests := []struct {
		name string
		file string
		want []Entry
	}{
		{
			name: "crlf",
			file: "crlf.env",
			want: []Entry{{Key: "A", Value: "1", Line: 1}, {Key: "B", Value: "two", Line: 2}},
		},
		{
			name: "bom",
			file: "bom.env",
			want: []Entry{{Key: "FIRST", Value: "1", Line: 1}, {Key: "SECOND", Value: "2", Line: 2}},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			path := filepath.Join("testdata", tt.file)
			f, err := ParseFile(path)
			if err != nil {
				t.Fatalf("ParseFile() error = %v", err)
			}
			if f.Path != path {
				t.Errorf("Path = %q, want %q", f.Path, path)
			}
			if !reflect.DeepEqual(f.Entries, tt.want) {
				t.Errorf("Entries = %+v, want %+v", f.Entries, tt.want)
			}
		})
	}
}

func TestParseFileErrors(t *testing.T) {
	if _, err := ParseFile(filepath.Join("testdata", "missing.env")); err == nil {
		t.Error("ParseFile(missing) error = nil, want error")
	}

	path := filepath.Join("testdata", "malformed.env")
	_, err := ParseFile(path)
	perr, ok := err.(*ParseError)
	if !ok {
		t.Fatalf("ParseFile(malformed) error type = %T, want *ParseError", err)
	}
	if perr.Path != path || perr.Line != 2 {
		t.Errorf("ParseError = %+v, want Path %q Line 2", perr, path)
	}
}

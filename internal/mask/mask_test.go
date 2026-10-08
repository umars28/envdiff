package mask

import (
	"regexp"
	"strings"
	"testing"
)

var format = regexp.MustCompile(`^<masked:[0-9a-f]{6}>$`)

func TestFingerprintFormat(t *testing.T) {
	m := NewWithSalt([]byte("salt"))
	for _, value := range []string{"", "8080", "postgres://user:pw@host/db", strings.Repeat("x", 4096)} {
		got := m.Fingerprint(value)
		if !format.MatchString(got.String()) {
			t.Errorf("Fingerprint(%.12q) = %q, want it to match %s", value, got, format)
		}
	}
}

func TestFingerprintDeterministicWithinSalt(t *testing.T) {
	a := NewWithSalt([]byte("salt"))
	b := NewWithSalt([]byte("salt"))
	if a.Fingerprint("value") != b.Fingerprint("value") {
		t.Errorf("same salt and value gave %q and %q, want equal", a.Fingerprint("value"), b.Fingerprint("value"))
	}
	if a.Fingerprint("one") == a.Fingerprint("two") {
		t.Error("different values collided under the same salt")
	}
}

func TestFingerprintDivergesAcrossSalts(t *testing.T) {
	a := NewWithSalt([]byte("salt-a"))
	b := NewWithSalt([]byte("salt-b"))
	if a.Fingerprint("value") == b.Fingerprint("value") {
		t.Error("different salts produced the same fingerprint")
	}
}

func TestFingerprintLeaksNoInput(t *testing.T) {
	const value = "hunter2-TOTALLY-SECRET"
	got := NewWithSalt([]byte("salt")).Fingerprint(value).String()
	for i := 0; i+3 <= len(value); i++ {
		if strings.Contains(got, value[i:i+3]) {
			t.Fatalf("Fingerprint output %q contains input substring %q", got, value[i:i+3])
		}
	}
}

func TestNewSaltIsRandom(t *testing.T) {
	a, err := New()
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	b, err := New()
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	if a.Fingerprint("value") == b.Fingerprint("value") {
		t.Error("two Maskers from New() share a salt")
	}
}

func TestUnknown(t *testing.T) {
	if Unknown.String() != "<masked:------>" {
		t.Errorf("Unknown = %q, want %q", Unknown, "<masked:------>")
	}
}

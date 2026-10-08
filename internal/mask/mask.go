package mask

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
)

type Print string

const Unknown Print = "<masked:------>"

const saltLen = 32

type Masker struct {
	salt []byte
}

func New() (*Masker, error) {
	salt := make([]byte, saltLen)
	if _, err := rand.Read(salt); err != nil {
		return nil, fmt.Errorf("generate salt: %w", err)
	}
	return NewWithSalt(salt), nil
}

func NewWithSalt(salt []byte) *Masker {
	return &Masker{salt: salt}
}

func (m *Masker) Fingerprint(value string) Print {
	mac := hmac.New(sha256.New, m.salt)
	mac.Write([]byte(value))
	return Print(fmt.Sprintf("<masked:%s>", hex.EncodeToString(mac.Sum(nil))[:6]))
}

func (p Print) String() string {
	return string(p)
}

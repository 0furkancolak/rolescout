package auth

import (
	"bytes"
	"strings"
	"testing"
)

func TestHashAndVerifyPassword(t *testing.T) {
	hash, err := HashPassword("correct horse battery")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(hash, "$argon2id$v=19$m=65536,t=3,p=2$") {
		t.Fatalf("unexpected hash format: %s", hash)
	}
	if ok, err := VerifyPassword("correct horse battery", hash); err != nil || !ok {
		t.Fatalf("correct password rejected: %v", err)
	}
	if ok, _ := VerifyPassword("correct horse battenr", hash); ok {
		t.Fatal("wrong password accepted")
	}

	other, _ := HashPassword("correct horse battery")
	if other == hash {
		t.Fatal("hashes must be salted")
	}
}

func TestVerifyRejectsMalformedHashes(t *testing.T) {
	for _, h := range []string{"", "plain", "$argon2i$v=19$m=1,t=1,p=1$c2FsdA$a2V5", "$argon2id$v=19$m=x$c2FsdA$a2V5"} {
		if _, err := VerifyPassword("x", h); err == nil {
			t.Errorf("hash %q accepted", h)
		}
	}
}

func TestValidatePassword(t *testing.T) {
	if msg := ValidatePassword("short"); msg == "" {
		t.Error("short password accepted")
	}
	if msg := ValidatePassword(strings.Repeat("ş", 12)); msg != "" {
		t.Errorf("12 multi-byte characters rejected: %s", msg)
	}
	if msg := ValidatePassword(strings.Repeat("a", 129)); msg == "" {
		t.Error("overlong password accepted")
	}
}

func TestSessionTokens(t *testing.T) {
	token, hash, err := NewSessionToken()
	if err != nil {
		t.Fatal(err)
	}
	if len(token) != 43 || len(hash) != 32 {
		t.Fatalf("token length %d, hash length %d", len(token), len(hash))
	}
	if !bytes.Equal(hash, HashSessionToken(token)) {
		t.Fatal("hash does not match token")
	}
	other, _, _ := NewSessionToken()
	if other == token {
		t.Fatal("tokens must be random")
	}
}

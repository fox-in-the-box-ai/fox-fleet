package sessiontoken

import (
	"crypto/rand"
	"encoding/base64"
	"testing"
	"time"
)

func testKey() []byte {
	key := make([]byte, 32)
	if _, err := rand.Read(key); err != nil {
		panic(err)
	}
	return key
}

func TestRoundTrip(t *testing.T) {
	s := NewSigner(testKey())
	token, expiry, err := s.Generate(PurposeSSE, 10*time.Minute)
	if err != nil {
		t.Fatal(err)
	}
	if token == "" {
		t.Fatal("empty token")
	}
	if expiry.IsZero() {
		t.Fatal("zero expiry")
	}
	if err := s.Validate(token, PurposeSSE); err != nil {
		t.Fatalf("valid token rejected: %v", err)
	}
}

func TestExpiredToken(t *testing.T) {
	s := NewSigner(testKey())
	token, _, err := s.Generate(PurposeSSE, -1*time.Second)
	if err != nil {
		t.Fatal(err)
	}
	if err := s.Validate(token, PurposeSSE); err != ErrExpiredToken {
		t.Fatalf("expected ErrExpiredToken, got %v", err)
	}
}

func TestWrongPurpose(t *testing.T) {
	s := NewSigner(testKey())
	token, _, err := s.Generate(PurposeSSE, 10*time.Minute)
	if err != nil {
		t.Fatal(err)
	}
	if err := s.Validate(token, 0x99); err != ErrWrongPurpose {
		t.Fatalf("expected ErrWrongPurpose, got %v", err)
	}
}

func TestTamperedPayload(t *testing.T) {
	s := NewSigner(testKey())
	token, _, err := s.Generate(PurposeSSE, 10*time.Minute)
	if err != nil {
		t.Fatal(err)
	}
	raw, _ := base64.RawURLEncoding.DecodeString(token)
	raw[0] ^= 0xFF
	tampered := base64.RawURLEncoding.EncodeToString(raw)
	if err := s.Validate(tampered, PurposeSSE); err != ErrInvalidToken {
		t.Fatalf("expected ErrInvalidToken, got %v", err)
	}
}

func TestTamperedHMAC(t *testing.T) {
	s := NewSigner(testKey())
	token, _, err := s.Generate(PurposeSSE, 10*time.Minute)
	if err != nil {
		t.Fatal(err)
	}
	raw, _ := base64.RawURLEncoding.DecodeString(token)
	raw[len(raw)-1] ^= 0xFF
	tampered := base64.RawURLEncoding.EncodeToString(raw)
	if err := s.Validate(tampered, PurposeSSE); err != ErrInvalidToken {
		t.Fatalf("expected ErrInvalidToken, got %v", err)
	}
}

func TestWrongKey(t *testing.T) {
	s1 := NewSigner(testKey())
	s2 := NewSigner(testKey())
	token, _, err := s1.Generate(PurposeSSE, 10*time.Minute)
	if err != nil {
		t.Fatal(err)
	}
	if err := s2.Validate(token, PurposeSSE); err != ErrInvalidToken {
		t.Fatalf("expected ErrInvalidToken, got %v", err)
	}
}

func TestTooShort(t *testing.T) {
	s := NewSigner(testKey())
	if err := s.Validate("abc", PurposeSSE); err != ErrInvalidToken {
		t.Fatalf("expected ErrInvalidToken, got %v", err)
	}
}

func TestEmptyString(t *testing.T) {
	s := NewSigner(testKey())
	if err := s.Validate("", PurposeSSE); err != ErrInvalidToken {
		t.Fatalf("expected ErrInvalidToken, got %v", err)
	}
}

func TestAdminSecretRejected(t *testing.T) {
	s := NewSigner(testKey())
	adminSecret := "a1b2c3d4e5f6a1b2c3d4e5f6a1b2c3d4e5f6a1b2c3d4e5f6a1b2c3d4e5f6a1b2"
	if err := s.Validate(adminSecret, PurposeSSE); err != ErrInvalidToken {
		t.Fatalf("expected ErrInvalidToken for admin secret, got %v", err)
	}
}

func TestUniqueTokens(t *testing.T) {
	s := NewSigner(testKey())
	seen := make(map[string]bool)
	for i := 0; i < 100; i++ {
		token, _, err := s.Generate(PurposeSSE, 10*time.Minute)
		if err != nil {
			t.Fatal(err)
		}
		if seen[token] {
			t.Fatal("duplicate token generated")
		}
		seen[token] = true
	}
}

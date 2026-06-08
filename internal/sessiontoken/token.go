package sessiontoken

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/binary"
	"errors"
	"fmt"
	"time"
)

const (
	PurposeSSE byte = 0x01

	purposeLen = 1
	expiryLen  = 8
	nonceLen   = 16
	payloadLen = purposeLen + expiryLen + nonceLen
	hmacLen    = sha256.Size
	tokenLen   = payloadLen + hmacLen
)

var (
	ErrInvalidToken = errors.New("invalid session token")
	ErrExpiredToken = errors.New("session token expired")
	ErrWrongPurpose = errors.New("session token purpose mismatch")
)

type Signer struct {
	key []byte
}

func NewSigner(key []byte) *Signer {
	return &Signer{key: key}
}

func (s *Signer) Generate(purpose byte, ttl time.Duration) (string, time.Time, error) {
	expiry := time.Now().Add(ttl)

	payload := make([]byte, payloadLen)
	payload[0] = purpose
	binary.BigEndian.PutUint64(payload[purposeLen:], uint64(expiry.Unix()))
	if _, err := rand.Read(payload[purposeLen+expiryLen:]); err != nil {
		return "", time.Time{}, fmt.Errorf("sessiontoken: generate nonce: %w", err)
	}

	mac := hmac.New(sha256.New, s.key)
	mac.Write(payload)
	sig := mac.Sum(nil)

	raw := make([]byte, tokenLen)
	copy(raw, payload)
	copy(raw[payloadLen:], sig)

	encoded := base64.RawURLEncoding.EncodeToString(raw)
	return encoded, expiry, nil
}

func (s *Signer) Validate(encoded string, expectedPurpose byte) error {
	raw, err := base64.RawURLEncoding.DecodeString(encoded)
	if err != nil || len(raw) != tokenLen {
		return ErrInvalidToken
	}

	payload := raw[:payloadLen]
	sig := raw[payloadLen:]

	mac := hmac.New(sha256.New, s.key)
	mac.Write(payload)
	if !hmac.Equal(sig, mac.Sum(nil)) {
		return ErrInvalidToken
	}

	purpose := payload[0]
	if purpose != expectedPurpose {
		return ErrWrongPurpose
	}

	expirySec := binary.BigEndian.Uint64(payload[purposeLen:])
	if time.Now().Unix() > int64(expirySec) {
		return ErrExpiredToken
	}

	return nil
}

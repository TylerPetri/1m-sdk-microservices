package password

import (
	"crypto/rand"
	"crypto/subtle"
	"encoding/base64"
	"errors"
	"fmt"
	"strings"

	"golang.org/x/crypto/argon2"
)

// We encode hashes using the PHC string format:
//
// 	$argon2id$v=19$m=65536,t=3,p=2$<salt_b64>$<hash_b64>
//
// This makes it easy to tune parameters later while keeping verification stable.

const (
	argon2Version = 19

	// Parameters chosen as a solid default for online auth on modern servers.
	// Must tune with production profiling + latency budget.
	memoryKiB = 64 * 1024
	timeCost  = 3
	threads   = 2
	keyLen    = 32

	saltLen = 16
)

var (
	ErrInvalidHash = errors.New("invalid password hash")
	ErrMismatch    = errors.New("password mismatch")
)

func Hash(plaintext string) (string, error) {
	if plaintext == "" {
		return "", errors.New("empty password")
	}

	salt := make([]byte, saltLen)
	if _, err := rand.Read(salt); err != nil {
		return "", fmt.Errorf("rand salt: %w", err)
	}

	hash := argon2.IDKey([]byte(plaintext), salt, timeCost, memoryKiB, threads, keyLen)

	b64Salt := base64.RawStdEncoding.EncodeToString(salt)
	b64Hash := base64.RawStdEncoding.EncodeToString(hash)

	encoded := fmt.Sprintf("$argon2id$v=%d$m=%d,t=%d,p=%d$%s$%s",
		argon2Version, memoryKiB, timeCost, threads, b64Salt, b64Hash,
	)
	return encoded, nil
}

func Verify(plaintext, encoded string) error {
	p, err := parse(encoded)
	if err != nil {
		return err
	}

	computed := argon2.IDKey([]byte(plaintext), p.salt, p.timeCost, p.memoryKiB, p.threads, uint32(len(p.hash)))
	if subtle.ConstantTimeCompare(computed, p.hash) != 1 {
		return ErrMismatch
	}
	return nil
}

type parsed struct {
	memoryKiB uint32
	timeCost  uint32
	threads   uint8
	salt      []byte
	hash      []byte
}

func parse(encoded string) (*parsed, error) {
	fields := strings.Split(encoded, "$")

	// Expect: "" "argon2id" "v=19" "m=...,t=...,p=..." "salt" "hash"
	if len(fields) != 6 || fields[1] != "argon2id" {
		return nil, ErrInvalidHash
	}

	var version int
	if _, err := fmt.Sscanf(fields[2], "v=%d", &version); err != nil || version != argon2Version {
		return nil, ErrInvalidHash
	}

	var m, t, p int
	if _, err := fmt.Sscanf(fields[3], "m=%d,t=%d,p=%d", &m, &t, &p); err != nil {
		return nil, ErrInvalidHash
	}

	salt, err := base64.RawStdEncoding.DecodeString(fields[4])
	if err != nil || len(salt) < 8 {
		return nil, ErrInvalidHash
	}

	hash, err := base64.RawStdEncoding.DecodeString(fields[5])
	if err != nil || len(hash) < 16 {
		return nil, ErrInvalidHash
	}

	return &parsed{
		memoryKiB: uint32(m),
		timeCost:  uint32(t),
		threads:   uint8(p),
		salt:      salt,
		hash:      hash,
	}, nil
}

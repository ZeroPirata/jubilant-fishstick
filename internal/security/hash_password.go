package security

import (
	"crypto/rand"
	"crypto/subtle"
	"encoding/base64"
	"errors"
	"fmt"
	"hackton-treino/config"
	"strings"

	"golang.org/x/crypto/argon2"
)

var (
	ErrInvalidHash         = errors.New("the encoded hash is not in the correct format")
	ErrIncompatibleVersion = errors.New("incompatible version of argon2")
)

type Hasher struct {
	cfg       config.HashConfig
	semaphore chan struct{}
}

func NewHasher(cfg config.HashConfig) *Hasher {
	return &Hasher{
		cfg:       cfg,
		semaphore: make(chan struct{}, cfg.Argon2Parallelism),
	}
}

func (h *Hasher) Hash(password string) (string, error) {
	h.semaphore <- struct{}{}
	defer func() { <-h.semaphore }()

	salt := make([]byte, h.cfg.Argon2SaltLen)
	if _, err := rand.Read(salt); err != nil {
		return "", err
	}

	hash := argon2.IDKey(
		[]byte(password+h.cfg.Argon2Pepper),
		salt,
		h.cfg.Argon2Iterations,
		h.cfg.Argon2Memory,
		h.cfg.Argon2Parallelism,
		h.cfg.Argon2KeyLen,
	)

	b64Salt := base64.RawStdEncoding.EncodeToString(salt)
	b64Hash := base64.RawStdEncoding.EncodeToString(hash)

	encoded := fmt.Sprintf("$argon2id$v=%d$m=%d,t=%d,p=%d$%s$%s",
		argon2.Version, h.cfg.Argon2Memory, h.cfg.Argon2Iterations, h.cfg.Argon2Parallelism, b64Salt, b64Hash)

	return encoded, nil
}

func (h *Hasher) Verify(password, encodedHash string) (bool, error) {
	parts := strings.Split(encodedHash, "$")
	// Expected: ["", "argon2id", "v=19", "m=65536,t=3,p=2", "<salt>", "<hash>"]
	if len(parts) != 6 {
		return false, ErrInvalidHash
	}

	var version int
	if _, err := fmt.Sscanf(parts[2], "v=%d", &version); err != nil {
		return false, ErrInvalidHash
	}
	if version != argon2.Version {
		return false, ErrIncompatibleVersion
	}

	var memory, iterations uint32
	var parallelism uint8
	if _, err := fmt.Sscanf(parts[3], "m=%d,t=%d,p=%d", &memory, &iterations, &parallelism); err != nil {
		return false, ErrInvalidHash
	}

	salt, err := base64.RawStdEncoding.DecodeString(parts[4])
	if err != nil {
		return false, ErrInvalidHash
	}

	storedHash, err := base64.RawStdEncoding.DecodeString(parts[5])
	if err != nil {
		return false, ErrInvalidHash
	}

	h.semaphore <- struct{}{}
	defer func() { <-h.semaphore }()

	computedHash := argon2.IDKey(
		[]byte(password+h.cfg.Argon2Pepper),
		salt,
		iterations,
		memory,
		parallelism,
		uint32(len(storedHash)),
	)

	return subtle.ConstantTimeCompare(storedHash, computedHash) == 1, nil
}

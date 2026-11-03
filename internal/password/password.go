// Package password implements the login design spec §6 password module:
// argon2id PHC hashing, password policy validation, and history-reuse
// checking. It depends on no external services.
package password

import (
	"fmt"

	"github.com/alexedwards/argon2id"
)

// hashParams match login design spec §6: m=64MiB, t=3, p=4, salt 16B, key 32B.
var hashParams = &argon2id.Params{
	Memory:      64 * 1024,
	Iterations:  3,
	Parallelism: 4,
	SaltLength:  16,
	KeyLength:   32,
}

// Hash encodes password as a PHC argon2id string carrying hashParams.
func Hash(password string) (string, error) {
	hash, err := argon2id.CreateHash(password, hashParams)
	if err != nil {
		return "", fmt.Errorf("argon2id create hash: %w", err)
	}
	return hash, nil
}

// Verify checks password against a PHC argon2id hash, returning whether it
// matches. The hash's own stored params (per-PHC) are used for verification.
func Verify(password, hash string) (bool, error) {
	match, err := argon2id.ComparePasswordAndHash(password, hash)
	if err != nil {
		return false, fmt.Errorf("argon2id compare: %w", err)
	}
	return match, nil
}

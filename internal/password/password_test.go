package password

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func Test_Hash_returns_argon2id_PHC_with_spec_params(t *testing.T) {
	// When
	hash, err := Hash("correct horse battery staple")

	// Then
	require.NoError(t, err)
	require.True(t, strings.HasPrefix(hash, "$argon2id$v=19$m=65536,t=3,p=4$"),
		"hash %q does not carry the spec params m=64MiB t=3 p=4", hash)
	parts := strings.Split(strings.TrimPrefix(hash, "$"), "$")
	require.Len(t, parts, 5, "PHC string should be $argon2id$v=19$params$salt$hash")
	require.Equal(t, "argon2id", parts[0])
	require.Equal(t, "v=19", parts[1])
	require.Equal(t, "m=65536,t=3,p=4", parts[2])
	require.Len(t, parts[3], 22, "16-byte salt must encode to 22 unpadded base64 chars")
	require.Len(t, parts[4], 43, "32-byte key must encode to 43 unpadded base64 chars")
}

func Test_Hash_then_Verify_roundtrip(t *testing.T) {
	// Given
	const plain = "Correct Horse Battery Staple!9"
	hash, err := Hash(plain)
	require.NoError(t, err)

	// When
	ok, err := Verify(plain, hash)

	// Then
	require.NoError(t, err)
	require.True(t, ok)
}

func Test_Verify_rejects_wrong_password(t *testing.T) {
	// Given
	hash, err := Hash("correct horse battery staple")
	require.NoError(t, err)

	// When
	ok, err := Verify("wrong password", hash)

	// Then
	require.NoError(t, err)
	require.False(t, ok)
}

func Test_Verify_returns_error_for_malformed_hash(t *testing.T) {
	// When
	ok, err := Verify("whatever", "not-a-phc-hash")

	// Then
	require.Error(t, err)
	require.False(t, ok)
}

func Test_Hash_uses_a_fresh_salt_per_call(t *testing.T) {
	// Given
	const plain = "same password 123"

	// When
	h1, err := Hash(plain)
	require.NoError(t, err)
	h2, err := Hash(plain)
	require.NoError(t, err)

	// Then
	require.NotEqual(t, h1, h2, "two hashes of the same password must differ in salt")
	ok, err := Verify(plain, h2)
	require.NoError(t, err)
	require.True(t, ok)
}

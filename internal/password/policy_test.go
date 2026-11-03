package password

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func Test_Validate_accepts_strong_passwords(t *testing.T) {
	tests := []struct {
		name     string
		password string
		username string
	}{
		{"two classes at 12 chars", "Abcdef12345!", "alice"},
		{"all four classes", "Correct-Horse42!", "alice"},
		{"passphrase with symbols", "correct-horse-battery-staple-42!", "alice"},
		{"single class at 16 chars", strings.Repeat("a", 16), "alice"},
		{"unrelated to username", "Za1!zxcvbnm123", "alice"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// When
			err := Validate(tt.password, tt.username)

			// Then
			require.NoError(t, err)
		})
	}
}

func Test_Validate_rejects_too_short(t *testing.T) {
	tests := []struct {
		name      string
		password  string
		minLength int
	}{
		{"multi-class below 12", "Abcdef1234!", 12},
		{"single class below 16", strings.Repeat("a", 15), 16},
		{"empty", "", 16},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// When
			err := Validate(tt.password, "alice")

			// Then
			require.ErrorIs(t, err, ErrTooShort)
			var ts *TooShortError
			require.ErrorAs(t, err, &ts)
			require.Equal(t, tt.minLength, ts.MinLength)
		})
	}
}

func Test_Validate_rejects_password_equal_to_or_containing_username(t *testing.T) {
	tests := []struct {
		name     string
		password string
		username string
	}{
		{"equal to username", "alice12345678", "alice12345678"},
		{"contains username", "aliceSecret12!a", "alice"},
		{"contains username case-insensitively", "ALICESecret12!a", "alice"},
		{"username case-insensitive", "aliceSecret12!a", "ALICE"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// When
			err := Validate(tt.password, tt.username)

			// Then
			require.ErrorIs(t, err, ErrContainsUsername)
		})
	}
}

func Test_Validate_rejects_common_weak_passwords(t *testing.T) {
	tests := []struct {
		name     string
		password string
	}{
		{"long weak password", "password123456"},
		{"weak password mixed case", "PASSWORD123456"},
		{"twelve-char weak password", "qwerty123456"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// When
			err := Validate(tt.password, "alice")

			// Then
			require.ErrorIs(t, err, ErrTooCommon)
		})
	}
}

func Test_Validate_weak_list_has_exactly_100_entries(t *testing.T) {
	// Then
	require.Len(t, weakPasswords, 100, "embedded weak-password list must hold exactly 100 entries")
}

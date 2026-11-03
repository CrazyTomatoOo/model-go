package password

import (
	"errors"
	"fmt"
	"strings"
	"unicode"
	"unicode/utf8"
)

// Policy errors from login design spec §6.
var (
	ErrTooShort         = errors.New("password too short")
	ErrContainsUsername = errors.New("password must not equal or contain the username")
	ErrTooCommon        = errors.New("password is too common")
)

// TooShortError carries the minimum length that applied to the rejected
// password: 12 when the password uses at least two character classes,
// 16 for a single (or no) class.
type TooShortError struct {
	MinLength int
}

func (e *TooShortError) Error() string {
	return fmt.Sprintf("%v: minimum length is %d", ErrTooShort, e.MinLength)
}

func (e *TooShortError) Unwrap() error { return ErrTooShort }

// Validate checks password against the spec §6 policy. username may be
// empty; otherwise equality or case-insensitive containment is rejected.
func Validate(password, username string) error {
	minLength := 12
	if classes(password) < 2 {
		minLength = 16
	}
	if utf8.RuneCountInString(password) < minLength {
		return &TooShortError{MinLength: minLength}
	}
	if username != "" && strings.Contains(strings.ToLower(password), strings.ToLower(username)) {
		return ErrContainsUsername
	}
	if _, common := weakPasswords[strings.ToLower(password)]; common {
		return ErrTooCommon
	}
	return nil
}

func classes(s string) int {
	var lower, upper, digit, symbol bool
	for _, r := range s {
		switch {
		case unicode.IsLower(r):
			lower = true
		case unicode.IsUpper(r):
			upper = true
		case unicode.IsDigit(r):
			digit = true
		default:
			symbol = true
		}
	}
	count := 0
	for _, present := range []bool{lower, upper, digit, symbol} {
		if present {
			count++
		}
	}
	return count
}

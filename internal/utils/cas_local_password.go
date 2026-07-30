package utils

import (
	"strings"
	"unicode"
)

// CASLocalPlainPassword is the plaintext local password stored for CAS-synced users
// (CAS handles login; DB still keeps a bcrypt hash — see cas_auth.go).
// Rule: take the first four runes of username, pad on the RIGHT with ASCII '0' if shorter than 4,
// then if those four code points are all Unicode digits replace the first with ASCII 'Z',
// concatenated with the last four numeric digits extracted from mobilePhone (ASCII 0–9 only).
// When fewer than four digits remain after stripping, left-pad with '0' to four characters.
// When mobile yields no digits, the suffix is "0000".
// Result length is always 8 (typically letters/digits mixed when username contains letters).
func CASLocalPlainPassword(username string, mobilePhone string) string {
	return casUsernamePrefixFour(username) + casMobileDigitsLastFour(mobilePhone)
}

func casUsernamePrefixFour(username string) string {
	const width = 4
	var b strings.Builder
	n := 0
	for _, r := range username {
		if n >= width {
			break
		}
		b.WriteRune(r)
		n++
	}
	for n < width {
		b.WriteByte('0')
		n++
	}
	return casReplaceLeadingDigitPrefixIfAllNumeric(b.String())
}

// casReplaceLeadingDigitPrefixIfAllNumeric turns a 4-rune username prefix whose runes are
// all unicode.IsDigit into one whose first rune is 'Z' so the concatenated CAS password retains a letter.
func casReplaceLeadingDigitPrefixIfAllNumeric(prefixFour string) string {
	rs := []rune(prefixFour)
	if len(rs) != 4 {
		return prefixFour
	}
	for _, r := range rs {
		if !unicode.IsDigit(r) {
			return prefixFour
		}
	}
	rs[0] = 'Z'
	return string(rs)
}

func casMobileDigitsLastFour(mobilePhone string) string {
	digitsOnly := strings.Map(func(r rune) rune {
		if r >= '0' && r <= '9' {
			return r
		}
		return -1
	}, mobilePhone)
	if len(digitsOnly) >= 4 {
		return digitsOnly[len(digitsOnly)-4:]
	}
	s := digitsOnly
	for len(s) < 4 {
		s = "0" + s
	}
	return s
}

// ValidatePasswordLettersAndDigitsMinimum8 checks product rule:
// password length ≥ 8, and contains at least one letter + one decimal digit (Unicode aware).
func ValidatePasswordLettersAndDigitsMinimum8(password string) bool {
	if len(password) < 8 {
		return false
	}
	hasLetter := false
	hasDigit := false
	for _, r := range password {
		switch {
		case unicode.IsDigit(r):
			hasDigit = true
		case unicode.IsLetter(r):
			hasLetter = true
		}
		if hasDigit && hasLetter {
			return true
		}
	}
	return false
}

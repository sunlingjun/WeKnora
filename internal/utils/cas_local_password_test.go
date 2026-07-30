package utils

import "testing"

func TestCASLocalPlainPassword(t *testing.T) {
	t.Parallel()
	cases := []struct {
		username string
		mobile   string
		want     string
	}{
		{"zhangsan", "13812345678", "zhan5678"},
		{"ab", "", "ab000000"}, // padded username + mobile default 0000
		{"好用户名称", "166****8186", "好用户名8186"}, // four runes: 好/用/户/名
		{"x", "12", "x0000012"}, // mobile digits left-pad to 4
		{"", "99998888", "Z0008888"}, // padded "0000" all digits → Z000 + mobile last 4
		{"123456", "13900001111", "Z2341111"}, // account prefix numeric → Z234
	}
	for _, tc := range cases {
		if got := CASLocalPlainPassword(tc.username, tc.mobile); got != tc.want {
			t.Errorf("CASLocalPlainPassword(%q,%q)=%q want %q", tc.username, tc.mobile, got, tc.want)
		}
	}
}

func TestValidatePasswordLettersAndDigitsMinimum8(t *testing.T) {
	t.Parallel()
	if ValidatePasswordLettersAndDigitsMinimum8("short1") {
		t.Fatal("too short expected false")
	}
	if ValidatePasswordLettersAndDigitsMinimum8("abcdefgh") {
		t.Fatal("no digit expected false")
	}
	if ValidatePasswordLettersAndDigitsMinimum8("12345678") {
		t.Fatal("no letter expected false")
	}
	if !ValidatePasswordLettersAndDigitsMinimum8("abcd1234") {
		t.Fatal("valid password expected true")
	}
}

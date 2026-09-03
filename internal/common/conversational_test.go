package common

import "testing"

func TestIsTrivialConversationalQuery(t *testing.T) {
	t.Parallel()
	cases := []struct {
		q    string
		want bool
	}{
		{"你好", true},
		{"你好！", true},
		{"  Hello  ", true},
		{"hi", true},
		{"谢谢", true},
		{"此知识库内有哪些知识？", false},
		{"猪瘟怎么治", false},
		{"你好，猪瘟怎么治", false},
		{"", false},
	}
	for _, tc := range cases {
		if got := IsTrivialConversationalQuery(tc.q); got != tc.want {
			t.Fatalf("IsTrivialConversationalQuery(%q)=%v want %v", tc.q, got, tc.want)
		}
	}
}

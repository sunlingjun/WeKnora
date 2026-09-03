package common

import "testing"

func TestIsContentPolicyMessage(t *testing.T) {
	t.Parallel()
	cases := []struct {
		msg  string
		want bool
	}{
		{"", false},
		{"timeout", false},
		{`API error: 400 {"error":{"message":"Content Exists Risk"}}`, true},
		{"finish_reason=content_filter", true},
		{"DATA_INSPECTION_FAILED", true},
		{"risk control failed for deploy", false}, // intentionally not matched
	}
	for _, tc := range cases {
		if got := IsContentPolicyMessage(tc.msg); got != tc.want {
			t.Fatalf("IsContentPolicyMessage(%q)=%v want %v", tc.msg, got, tc.want)
		}
	}
}

func TestIsCatalogIntent(t *testing.T) {
	t.Parallel()
	cases := []struct {
		q    string
		want bool
	}{
		{"此知识库内有哪些知识？", true},
		{"列出文档", true},
		{"what documents are in this KB", true},
		{"学联网手册具体写了什么内容", false},
		{"帮我详细解读这份报告", false},
		{"你好", false},
		{"", false},
	}
	for _, tc := range cases {
		if got := IsCatalogIntent(tc.q); got != tc.want {
			t.Fatalf("IsCatalogIntent(%q)=%v want %v", tc.q, got, tc.want)
		}
	}
}

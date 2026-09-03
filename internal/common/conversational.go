package common

import (
	"strings"
	"unicode"
)

// trivialConversationalExact are whole-query greetings / thanks / farewells that
// should skip rewrite + KB retrieval and go straight to chat.
var trivialConversationalExact = map[string]struct{}{
	"你好": {}, "您好": {}, "你好呀": {}, "您好呀": {}, "你好啊": {}, "您好啊": {},
	"嗨": {}, "哈喽": {}, "hello": {}, "hi": {}, "hey": {}, "hola": {},
	"早": {}, "早上好": {}, "下午好": {}, "晚上好": {}, "午安": {},
	"good morning": {}, "good afternoon": {}, "good evening": {}, "good night": {},
	"在吗": {}, "在不在": {}, "你好吗": {}, "您好吗": {},
	"谢谢": {}, "谢谢你": {}, "谢谢您": {}, "多谢": {}, "thanks": {}, "thank you": {}, "thx": {},
	"再见": {}, "拜拜": {}, "回见": {}, "bye": {}, "goodbye": {}, "see you": {},
	"没事了": {}, "好的": {}, "好": {}, "嗯": {}, "ok": {}, "okay": {},
}

// IsTrivialConversationalQuery reports short greeting/chitchat turns that must
// not pay for query_understand + retrieval when knowledge bases are bound.
func IsTrivialConversationalQuery(query string) bool {
	q := strings.TrimSpace(query)
	if q == "" {
		return false
	}
	q = strings.TrimFunc(q, func(r rune) bool {
		return unicode.IsSpace(r) || strings.ContainsRune("！!？?。.～~…、，,", r)
	})
	if q == "" {
		return false
	}
	lower := strings.ToLower(q)
	if _, ok := trivialConversationalExact[lower]; ok {
		return true
	}
	if _, ok := trivialConversationalExact[q]; ok {
		return true
	}
	return false
}

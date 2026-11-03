package utils

import (
	"os"
	"testing"
	"time"
)

func TestMyDocTicketRoundTrip(t *testing.T) {
	prev := os.Getenv("SYSTEM_AES_KEY")
	t.Cleanup(func() {
		_ = os.Setenv("SYSTEM_AES_KEY", prev)
	})
	_ = os.Setenv("SYSTEM_AES_KEY", "0123456789abcdef0123456789abcdef")

	now := time.Unix(1_700_000_000, 0)
	ticket, exp, err := SignMyDocTicket("doc-1", "download", now)
	if err != nil {
		t.Fatalf("sign: %v", err)
	}
	if exp.Before(now) {
		t.Fatal("expires should be after now")
	}
	claims, err := ParseMyDocTicket(ticket, now.Add(time.Minute), false)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if claims.DocID != "doc-1" || claims.Purpose != "download" {
		t.Fatalf("claims=%+v", claims)
	}
	if _, err := ParseMyDocTicket(ticket, now.Add(10*time.Minute), false); err == nil {
		t.Fatal("expected expired")
	}
	if _, err := ParseMyDocTicket(ticket, now.Add(10*time.Minute), true); err != nil {
		t.Fatalf("expiredOK should allow: %v", err)
	}
	wrong, _, err := SignMyDocTicket("doc-2", "download", now)
	if err != nil {
		t.Fatalf("sign2: %v", err)
	}
	c2, err := ParseMyDocTicket(wrong, now, false)
	if err != nil {
		t.Fatalf("parse2: %v", err)
	}
	if c2.DocID == "doc-1" {
		t.Fatal("wrong doc should not match doc-1")
	}
	if HashMyDocTicketToken(ticket) == "" {
		t.Fatal("hash empty")
	}
}

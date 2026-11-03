package utils

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"strings"
	"time"
)

const (
	MyDocTicketPrefix = "mdt1."
	MyDocTicketTTL    = 5 * time.Minute
	MyDocTicketSkew   = 30 * time.Second
)

// MyDocTicketClaims is the verified payload of a MyDoc short ticket.
type MyDocTicketClaims struct {
	Purpose string
	DocID   string
	Expires int64
}

func canonicalMyDocTicket(purpose, docID string, expires int64) string {
	return fmt.Sprintf("purpose=%s&doc_id=%s&expires=%d", purpose, docID, expires)
}

// HashMyDocTicketToken returns a stable hex digest for persistence lookups.
func HashMyDocTicketToken(token string) string {
	sum := sha256.Sum256([]byte(token))
	return hex.EncodeToString(sum[:])
}

// SignMyDocTicket mints a short-lived HMAC ticket for preview/download.
func SignMyDocTicket(docID, purpose string, now time.Time) (ticket string, expiresAt time.Time, err error) {
	key := SystemHMACKey()
	if key == nil {
		return "", time.Time{}, fmt.Errorf("mydoc ticket: SYSTEM_AES_KEY not configured")
	}
	if now.IsZero() {
		now = time.Now()
	}
	expiresAt = now.Add(MyDocTicketTTL)
	canonical := canonicalMyDocTicket(purpose, docID, expiresAt.Unix())
	mac := hmac.New(sha256.New, key)
	mac.Write([]byte(canonical))
	sig := hex.EncodeToString(mac.Sum(nil))
	encoded := base64.RawURLEncoding.EncodeToString([]byte(canonical))
	return MyDocTicketPrefix + encoded + "." + sig, expiresAt, nil
}

// ParseMyDocTicket verifies HMAC and returns claims.
func ParseMyDocTicket(ticket string, now time.Time, expiredOK bool) (*MyDocTicketClaims, error) {
	key := SystemHMACKey()
	if key == nil {
		return nil, fmt.Errorf("mydoc ticket: SYSTEM_AES_KEY not configured")
	}
	if !strings.HasPrefix(ticket, MyDocTicketPrefix) {
		return nil, fmt.Errorf("invalid ticket")
	}
	rest := strings.TrimPrefix(ticket, MyDocTicketPrefix)
	dot := strings.LastIndex(rest, ".")
	if dot <= 0 || dot == len(rest)-1 {
		return nil, fmt.Errorf("invalid ticket")
	}
	encoded, sigHex := rest[:dot], rest[dot+1:]
	canonical, err := base64.RawURLEncoding.DecodeString(encoded)
	if err != nil {
		return nil, fmt.Errorf("invalid ticket")
	}
	mac := hmac.New(sha256.New, key)
	mac.Write(canonical)
	expected := hex.EncodeToString(mac.Sum(nil))
	if !hmac.Equal([]byte(sigHex), []byte(expected)) {
		return nil, fmt.Errorf("invalid ticket")
	}
	claims, err := parseMyDocCanonical(string(canonical))
	if err != nil {
		return nil, err
	}
	if now.IsZero() {
		now = time.Now()
	}
	if !expiredOK && now.Unix() > claims.Expires+int64(MyDocTicketSkew.Seconds()) {
		return nil, fmt.Errorf("ticket expired")
	}
	return claims, nil
}

func parseMyDocCanonical(canonical string) (*MyDocTicketClaims, error) {
	parts := strings.Split(canonical, "&")
	got := map[string]string{}
	for _, part := range parts {
		k, v, ok := strings.Cut(part, "=")
		if !ok {
			return nil, fmt.Errorf("invalid ticket")
		}
		got[k] = v
	}
	expires, err := parseInt64(got["expires"])
	if err != nil {
		return nil, fmt.Errorf("invalid ticket")
	}
	purpose := got["purpose"]
	docID := got["doc_id"]
	if purpose == "" || docID == "" {
		return nil, fmt.Errorf("invalid ticket")
	}
	return &MyDocTicketClaims{Purpose: purpose, DocID: docID, Expires: expires}, nil
}

func parseInt64(s string) (int64, error) {
	var n int64
	for _, c := range s {
		if c < '0' || c > '9' {
			return 0, fmt.Errorf("not an int")
		}
		n = n*10 + int64(c-'0')
	}
	return n, nil
}

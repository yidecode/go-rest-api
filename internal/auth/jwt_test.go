package auth

import (
	"testing"
	"time"
)

func TestIssueVerifyRoundTrip(t *testing.T) {
	tok, err := Issue("test-secret", "user-123", time.Hour)
	if err != nil {
		t.Fatalf("Issue: %v", err)
	}
	sub, err := Verify("test-secret", tok)
	if err != nil {
		t.Fatalf("Verify: %v", err)
	}
	if sub != "user-123" {
		t.Fatalf("got subject %q, want %q", sub, "user-123")
	}
}

func TestVerifyRejects(t *testing.T) {
	cases := map[string]string{
		"empty":         "",
		"garbage":       "not-a-token",
		"wrong secret":  mustIssue(t, "other-secret", "user-123", time.Hour),
		"expired token": mustIssue(t, "test-secret", "user-123", -time.Hour),
		"tampered":      mustIssue(t, "test-secret", "user-123", time.Hour) + "x",
	}
	for name, tok := range cases {
		t.Run(name, func(t *testing.T) {
			if _, err := Verify("test-secret", tok); err == nil {
				t.Fatal("expected error, got nil")
			}
		})
	}
}

func mustIssue(t *testing.T, secret, sub string, ttl time.Duration) string {
	t.Helper()
	tok, err := Issue(secret, sub, ttl)
	if err != nil {
		t.Fatalf("Issue: %v", err)
	}
	return tok
}

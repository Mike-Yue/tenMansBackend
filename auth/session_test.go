package auth

import (
	"testing"
	"time"
)

func testConfig() *Config {
	return &Config{
		SessionSecret: []byte("test-secret-key"),
		AppBaseURL:    "http://localhost:5173",
		CookieSecure:  false,
		SessionTTL:    time.Hour,
	}
}

func TestSessionRoundTrip(t *testing.T) {
	c := testConfig()
	const steamID int64 = 76561198137660080 // exceeds JS MAX_SAFE_INTEGER on purpose

	value := c.signSession(steamID, time.Now().Add(time.Hour).Unix())
	got, ok := c.parseSession(value)
	if !ok {
		t.Fatal("valid session was rejected")
	}
	if got != steamID {
		t.Fatalf("steamID = %d, want %d", got, steamID)
	}
}

func TestSessionTampered(t *testing.T) {
	c := testConfig()
	value := c.signSession(123, time.Now().Add(time.Hour).Unix())

	// Swap the signed SteamID; the signature no longer matches the payload.
	tampered := "999." + value[len("123."):]
	if _, ok := c.parseSession(tampered); ok {
		t.Fatal("tampered session was accepted")
	}
}

func TestSessionExpired(t *testing.T) {
	c := testConfig()
	value := c.signSession(123, time.Now().Add(-time.Minute).Unix())
	if _, ok := c.parseSession(value); ok {
		t.Fatal("expired session was accepted")
	}
}

func TestSessionWrongSecret(t *testing.T) {
	c := testConfig()
	value := c.signSession(123, time.Now().Add(time.Hour).Unix())

	other := testConfig()
	other.SessionSecret = []byte("a-different-secret")
	if _, ok := other.parseSession(value); ok {
		t.Fatal("session was accepted under the wrong secret")
	}
}

func TestSessionMalformed(t *testing.T) {
	c := testConfig()
	for _, v := range []string{"", "garbage", "1.2", "1.2.3.4"} {
		if _, ok := c.parseSession(v); ok {
			t.Fatalf("malformed session %q was accepted", v)
		}
	}
}

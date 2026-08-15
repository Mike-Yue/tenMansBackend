package auth

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"net/http"
	"strconv"
	"strings"
	"time"
)

// sessionCookieName is the name of the session cookie. It's host-only (no Domain
// attribute) so the browser scopes it to whatever origin it was set on — behind
// the frontend's rewrite proxy that's the frontend origin, making it first-party.
const sessionCookieName = "tm_session"

// signSession returns a signed token "<steamID>.<exp>.<sig>" where sig is an
// HMAC over "<steamID>.<exp>". All characters are cookie-safe.
func (c *Config) signSession(steamID, exp int64) string {
	payload := strconv.FormatInt(steamID, 10) + "." + strconv.FormatInt(exp, 10)
	return payload + "." + c.mac(payload)
}

// mac computes the hex HMAC-SHA256 of payload under the session secret.
func (c *Config) mac(payload string) string {
	m := hmac.New(sha256.New, c.SessionSecret)
	m.Write([]byte(payload))
	return hex.EncodeToString(m.Sum(nil))
}

// setSessionCookie issues a fresh signed session cookie for steamID.
func (c *Config) setSessionCookie(w http.ResponseWriter, steamID int64) {
	exp := time.Now().Add(c.SessionTTL).Unix()
	http.SetCookie(w, &http.Cookie{
		Name:     sessionCookieName,
		Value:    c.signSession(steamID, exp),
		Path:     "/",
		MaxAge:   int(c.SessionTTL / time.Second),
		HttpOnly: true,
		Secure:   c.CookieSecure,
		SameSite: http.SameSiteLaxMode,
	})
}

// clearSessionCookie expires the session cookie.
func (c *Config) clearSessionCookie(w http.ResponseWriter) {
	http.SetCookie(w, &http.Cookie{
		Name:     sessionCookieName,
		Value:    "",
		Path:     "/",
		MaxAge:   -1,
		HttpOnly: true,
		Secure:   c.CookieSecure,
		SameSite: http.SameSiteLaxMode,
	})
}

// readSession returns the SteamID from a valid session cookie on the request.
func (c *Config) readSession(r *http.Request) (int64, bool) {
	ck, err := r.Cookie(sessionCookieName)
	if err != nil {
		return 0, false
	}
	return c.parseSession(ck.Value)
}

// parseSession validates a signed token and returns its SteamID. It rejects
// tampered tokens (via a constant-time HMAC compare) and expired ones.
func (c *Config) parseSession(value string) (int64, bool) {
	parts := strings.Split(value, ".")
	if len(parts) != 3 {
		return 0, false
	}

	payload := parts[0] + "." + parts[1]
	if !hmac.Equal([]byte(c.mac(payload)), []byte(parts[2])) {
		return 0, false
	}

	exp, err := strconv.ParseInt(parts[1], 10, 64)
	if err != nil || time.Now().Unix() >= exp {
		return 0, false
	}

	steamID, err := strconv.ParseInt(parts[0], 10, 64)
	if err != nil {
		return 0, false
	}
	return steamID, true
}

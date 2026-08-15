package auth

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"strconv"
	"strings"
	"time"
)

// issueToken signs a fresh session token for steamID, valid for SessionTTL.
func (c *Config) issueToken(steamID int64) string {
	exp := time.Now().Add(c.SessionTTL).Unix()
	return c.signSession(steamID, exp)
}

// signSession returns a signed token "<steamID>.<exp>.<sig>" where sig is an
// HMAC over "<steamID>.<exp>". All characters are URL- and header-safe.
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

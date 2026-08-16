package auth

import (
	"encoding/json"
	"net/http"
	"net/url"
	"strconv"
	"sync"
	"time"
)

// The OpenID login only yields a numeric SteamID64, so resolving a human-readable
// display name requires a call to the Steam Web API. Results are cached briefly so
// /api/auth/me doesn't hit Steam on every page load.

const personaCacheTTL = 10 * time.Minute

type personaEntry struct {
	name      string
	expiresAt time.Time
}

var (
	personaMu    sync.Mutex
	personaCache = map[int64]personaEntry{}
)

var steamAPIClient = &http.Client{Timeout: 5 * time.Second}

// steamPersonaName returns the current Steam display name for steamID, or "" when
// no API key is configured or the lookup fails (callers then fall back to a stored
// username or the numeric ID). Successful lookups are cached for personaCacheTTL.
func (c *Config) steamPersonaName(steamID int64) string {
	if c.SteamAPIKey == "" {
		return ""
	}

	personaMu.Lock()
	if e, ok := personaCache[steamID]; ok && time.Now().Before(e.expiresAt) {
		personaMu.Unlock()
		return e.name
	}
	personaMu.Unlock()

	name := fetchSteamPersonaName(c.SteamAPIKey, steamID)
	if name != "" {
		personaMu.Lock()
		personaCache[steamID] = personaEntry{name: name, expiresAt: time.Now().Add(personaCacheTTL)}
		personaMu.Unlock()
	}
	return name
}

// fetchSteamPersonaName calls ISteamUser/GetPlayerSummaries and returns the
// player's personaname, or "" on any error (missing player, bad key, timeout).
func fetchSteamPersonaName(apiKey string, steamID int64) string {
	endpoint := "https://api.steampowered.com/ISteamUser/GetPlayerSummaries/v0002/?" +
		url.Values{
			"key":      {apiKey},
			"steamids": {strconv.FormatInt(steamID, 10)},
		}.Encode()

	resp, err := steamAPIClient.Get(endpoint)
	if err != nil {
		return ""
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return ""
	}

	var body struct {
		Response struct {
			Players []struct {
				PersonaName string `json:"personaname"`
			} `json:"players"`
		} `json:"response"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		return ""
	}
	if len(body.Response.Players) == 0 {
		return ""
	}
	return body.Response.Players[0].PersonaName
}

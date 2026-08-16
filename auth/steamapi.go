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
// display name and avatar requires a call to the Steam Web API. Results are cached
// briefly so /api/auth/me doesn't hit Steam on every page load.

const personaCacheTTL = 10 * time.Minute

// steamProfile is the subset of a player's Steam summary we surface to the app.
type steamProfile struct {
	Name   string
	Avatar string // avatarmedium URL (64x64), served from Steam's CDN
}

type profileEntry struct {
	profile   steamProfile
	expiresAt time.Time
}

var (
	profileMu    sync.Mutex
	profileCache = map[int64]profileEntry{}
)

var steamAPIClient = &http.Client{Timeout: 5 * time.Second}

// steamProfileFor returns the current Steam display name and avatar for steamID,
// or a zero steamProfile when no API key is configured or the lookup fails
// (callers then fall back to a stored username / the numeric ID). Successful
// lookups are cached for personaCacheTTL.
func (c *Config) steamProfileFor(steamID int64) steamProfile {
	if c.SteamAPIKey == "" {
		return steamProfile{}
	}

	profileMu.Lock()
	if e, ok := profileCache[steamID]; ok && time.Now().Before(e.expiresAt) {
		profileMu.Unlock()
		return e.profile
	}
	profileMu.Unlock()

	p := fetchSteamProfile(c.SteamAPIKey, steamID)
	if p.Name != "" {
		profileMu.Lock()
		profileCache[steamID] = profileEntry{profile: p, expiresAt: time.Now().Add(personaCacheTTL)}
		profileMu.Unlock()
	}
	return p
}

// fetchSteamProfile calls ISteamUser/GetPlayerSummaries and returns the player's
// persona name and avatar, or a zero steamProfile on any error (missing player,
// bad key, timeout).
func fetchSteamProfile(apiKey string, steamID int64) steamProfile {
	endpoint := "https://api.steampowered.com/ISteamUser/GetPlayerSummaries/v0002/?" +
		url.Values{
			"key":      {apiKey},
			"steamids": {strconv.FormatInt(steamID, 10)},
		}.Encode()

	resp, err := steamAPIClient.Get(endpoint)
	if err != nil {
		return steamProfile{}
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return steamProfile{}
	}

	var body struct {
		Response struct {
			Players []struct {
				PersonaName  string `json:"personaname"`
				AvatarMedium string `json:"avatarmedium"`
			} `json:"players"`
		} `json:"response"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		return steamProfile{}
	}
	if len(body.Response.Players) == 0 {
		return steamProfile{}
	}
	p := body.Response.Players[0]
	return steamProfile{Name: p.PersonaName, Avatar: p.AvatarMedium}
}

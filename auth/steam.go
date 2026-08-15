package auth

import (
	"errors"
	"net/url"
	"strconv"
	"strings"

	openid "github.com/yohcop/openid-go"
)

// steamOpenIDEndpoint is Steam's fixed OpenID 2.0 login endpoint. Steam does not
// support modern OIDC/OAuth2, only OpenID 2.0.
const steamOpenIDEndpoint = "https://steamcommunity.com/openid/login"

// identifierSelect is the OpenID 2.0 "let the OP choose the identity" sentinel.
const identifierSelect = "http://specs.openid.net/auth/2.0/identifier_select"

// nonceStore and discoveryCache are in-memory and package-level. This is fine for
// a single backend instance; if the service is ever scaled horizontally these
// must be replaced with a shared (e.g. DB/Redis-backed) implementation so a
// callback can land on any instance.
var (
	nonceStore     = openid.NewSimpleNonceStore()
	discoveryCache = openid.NewSimpleDiscoveryCache()
)

// steamLoginRedirectURL builds the URL to send the browser to for Steam login.
// realm is the app origin; returnTo must sit under it.
//
// We construct the checkid_setup URL by hand rather than via openid.RedirectURL
// because that helper performs Yadis discovery on the OP endpoint, and a bare GET
// to https://steamcommunity.com/openid/login 302-redirects to Steam's homepage
// (no provider link) — so discovery fails. Steam's OP endpoint is a fixed
// constant, so no discovery is needed here. (Verification on the callback still
// uses openid.Verify, whose discovery targets the claimed-id URL, which does
// serve a valid XRDS document.)
func steamLoginRedirectURL(returnTo, realm string) string {
	v := url.Values{}
	v.Set("openid.ns", "http://specs.openid.net/auth/2.0")
	v.Set("openid.mode", "checkid_setup")
	v.Set("openid.return_to", returnTo)
	v.Set("openid.realm", realm)
	v.Set("openid.identity", identifierSelect)
	v.Set("openid.claimed_id", identifierSelect)
	return steamOpenIDEndpoint + "?" + v.Encode()
}

// verifySteamLogin verifies the OpenID assertion carried by fullURL (the callback
// URL including Steam's openid.* query params) and returns the SteamID64.
func verifySteamLogin(fullURL string) (int64, error) {
	claimedID, err := openid.Verify(fullURL, discoveryCache, nonceStore)
	if err != nil {
		return 0, err
	}
	return steamIDFromClaimedID(claimedID)
}

// steamIDFromClaimedID extracts the SteamID64 from a claimed identifier like
// "https://steamcommunity.com/openid/id/76561198137660080".
func steamIDFromClaimedID(claimedID string) (int64, error) {
	i := strings.LastIndex(claimedID, "/")
	if i == -1 || i == len(claimedID)-1 {
		return 0, errors.New("unexpected steam claimed id: " + claimedID)
	}
	return strconv.ParseInt(claimedID[i+1:], 10, 64)
}

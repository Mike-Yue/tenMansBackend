package stats

// PlayerStats is a single user's aggregated stat line, optionally scoped to one
// season.
type PlayerStats struct {
	Kills         int64   `json:"kills"`
	Deaths        int64   `json:"deaths"`
	Assists       int64   `json:"assists"`
	MVPs          int64   `json:"mvps"`
	UtilityDamage int64   `json:"utilityDamage"`
	FlashAssists  int64   `json:"flashAssists"`
	// KDRatio is total kills / total deaths (kills when deaths=0).
	KDRatio float64 `json:"kdRatio"`
	// HeadshotPct is a fraction in [0,1] (headshot kills / kills).
	HeadshotPct float64 `json:"headshotPct"`
	// Winrate is a fraction in [0,1] (games won / games played); the frontend
	// decides how to format it as a percentage.
	Winrate float64 `json:"winrate"`
}

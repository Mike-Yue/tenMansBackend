package stats

// PlayerStats is the aggregated, all-time stat line for a single user.
type PlayerStats struct {
	Kills   int64   `json:"kills"`
	Deaths  int64   `json:"deaths"`
	Assists int64   `json:"assists"`
	MVPs    int64   `json:"mvps"`
	// Winrate is a fraction in [0,1] (games won / games played); the frontend
	// decides how to format it as a percentage.
	Winrate float64 `json:"winrate"`
}

package seasons

// Season maps to a row in the "seasons" table.
//
// start_at and end_at are stored as "YYYY-MM-DD" date strings; match season
// resolution (matches.CurrentSeasonID) compares them lexicographically against
// today in that same format, so the format matters and is validated on create.
type Season struct {
	ID      int64  `json:"id"`
	Name    string `json:"name"`
	StartAt string `json:"startAt"`
	EndAt   string `json:"endAt"`
}

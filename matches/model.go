package matches

// Match is the DB model: it mirrors a row in the "Matches" table and is used
// only by the data and service layers. It intentionally carries no json tags —
// how a match is exposed over the API is the handler's concern (see
// MatchResponse in handler.go).
//
// All columns are NOT NULL, so no pointer fields are needed. The processed
// column is stored as an INTEGER 0/1 and represented here as a bool.
type Match struct {
	ID          int64
	Map         string
	PlayedAt    string
	UploadedAt  string
	UploadHash  string
	Processed   bool
	SeasonID    int64
	TotalRounds int64
}

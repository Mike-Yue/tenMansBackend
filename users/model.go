package users

// User maps to a row in the "Users" table.
//
// steam_username is nullable in the schema, so it is modeled as a pointer:
// nil represents SQL NULL, which marshals to JSON null.
type User struct {
	ID            int64   `json:"id"`
	SteamID       int64   `json:"steamId"`
	SteamUsername *string `json:"steamUsername"`
	CreatedAt     string  `json:"createdAt"`
}

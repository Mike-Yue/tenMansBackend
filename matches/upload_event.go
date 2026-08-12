package matches

import (
	"bytes"
	_ "embed"
	"fmt"
	"math"
	"strconv"

	"github.com/santhosh-tekuri/jsonschema/v6"
)

// parserEventSchemaJSON is the TenMan match parser event v1 contract, embedded
// so the binary is self-contained. compiledEventSchema is compiled from it once
// at startup.
//
//go:embed parser_event.schema.json
var parserEventSchemaJSON []byte

var compiledEventSchema = mustCompileEventSchema()

// mustCompileEventSchema compiles the embedded parser-event contract. A failure
// here means the embedded schema is malformed, which is a build-time bug, so we
// panic rather than degrade at runtime.
func mustCompileEventSchema() *jsonschema.Schema {
	doc, err := jsonschema.UnmarshalJSON(bytes.NewReader(parserEventSchemaJSON))
	if err != nil {
		panic(fmt.Sprintf("matches: parse embedded parser event schema: %v", err))
	}
	c := jsonschema.NewCompiler()
	c.AssertFormat() // enforce "format" (e.g. date-time), not just annotate it
	const loc = "parser_event.schema.json"
	if err := c.AddResource(loc, doc); err != nil {
		panic(fmt.Sprintf("matches: add parser event schema resource: %v", err))
	}
	sch, err := c.Compile(loc)
	if err != nil {
		panic(fmt.Sprintf("matches: compile parser event schema: %v", err))
	}
	return sch
}

// validateUploadEvent checks raw against the parser-event contract. It returns a
// descriptive error when the payload is not valid JSON or does not satisfy the
// schema; callers translate that into a 400.
func validateUploadEvent(raw []byte) error {
	inst, err := jsonschema.UnmarshalJSON(bytes.NewReader(raw))
	if err != nil {
		return fmt.Errorf("invalid JSON: %w", err)
	}
	if err := compiledEventSchema.Validate(inst); err != nil {
		return err
	}
	return nil
}

// --- Typed view of the fields we consume (populated after validation) ---

// parserEvent is a partial, decode-only view of a parser event. Only the fields
// the ingest path uses are typed; the schema has already guaranteed the rest.
type parserEvent struct {
	EventType  string       `json:"eventType"`
	OccurredAt string       `json:"occurredAt"`
	Source     eventSource  `json:"source"`
	Result     *eventResult `json:"result"`
}

type eventSource struct {
	Key string `json:"key"`
}

type eventResult struct {
	MapName string      `json:"mapName"`
	Teams   []eventTeam `json:"teams"`
}

type eventTeam struct {
	TeamKey      string        `json:"teamKey"`
	StartingSide string        `json:"startingSide"`
	Score        int64         `json:"score"`
	Players      []eventPlayer `json:"players"`
}

type eventPlayer struct {
	SteamID     string           `json:"steamId"`
	DisplayName string           `json:"displayName"`
	Stats       eventPlayerStats `json:"stats"`
}

type eventPlayerStats struct {
	Kills         int64 `json:"kills"`
	Deaths        int64 `json:"deaths"`
	Assists       int64 `json:"assists"`
	MVPs          int64 `json:"mvps"`
	DamageAssists int64 `json:"damageAssists"`
	FlashAssists  int64 `json:"flashAssists"`
	HeadshotKills int64 `json:"headshotKills"`
	TotalDamage   int64 `json:"totalDamage"`
	UtilityDamage int64 `json:"utilityDamage"`
	RoundsPlayed  int64 `json:"roundsPlayed"`
}

// Event type discriminators from the contract.
const (
	eventTypeStarted   = "parser.job.started.v1"
	eventTypeSucceeded = "parser.job.succeeded.v1"
	eventTypeFailed    = "parser.job.failed.v1"
)

// userResolver maps a Steam ID (and display name, for auto-creation) to the
// internal user primary key used by the stats table.
type userResolver func(steamID int64, displayName string) (int64, error)

// toParseResult maps a succeeded event's result onto the existing ParseResult
// domain model. Player Steam IDs are resolved to internal user ids via resolve.
// The richer per-player stats, rounds, head-to-head and quality data have no
// columns in the current schema and are intentionally dropped.
func (e *parserEvent) toParseResult(resolve userResolver) (ParseResult, error) {
	res := e.Result
	if res == nil || len(res.Teams) != 2 {
		return ParseResult{}, fmt.Errorf("succeeded event missing two teams")
	}

	totalRounds := res.Teams[0].Score + res.Teams[1].Score

	teams := make([]NewTeam, 0, len(res.Teams))
	for i, t := range res.Teams {
		other := res.Teams[1-i]
		players := make([]NewPlayerStat, 0, len(t.Players))
		for _, p := range t.Players {
			steamID, err := strconv.ParseInt(p.SteamID, 10, 64)
			if err != nil {
				return ParseResult{}, fmt.Errorf("invalid steamId %q: %w", p.SteamID, err)
			}
			playerID, err := resolve(steamID, p.DisplayName)
			if err != nil {
				return ParseResult{}, err
			}
			players = append(players, NewPlayerStat{
				PlayerID:      playerID,
				Kills:         p.Stats.Kills,
				Deaths:        p.Stats.Deaths,
				Assists:       p.Stats.Assists,
				KDRatio:       kdRatio(p.Stats.Kills, p.Stats.Deaths),
				MVPs:          p.Stats.MVPs,
				DamageAssists: p.Stats.DamageAssists,
				FlashAssists:  p.Stats.FlashAssists,
				HeadshotKills: p.Stats.HeadshotKills,
				TotalDamage:   p.Stats.TotalDamage,
				UtilityDamage: p.Stats.UtilityDamage,
				RoundsPlayed:  p.Stats.RoundsPlayed,
			})
		}
		teams = append(teams, NewTeam{
			TeamSlot:     teamSlot(t.TeamKey),
			StartingSide: t.StartingSide,
			RoundsWon:    t.Score,
			Result:       matchResult(t.Score, other.Score),
			Players:      players,
		})
	}

	return ParseResult{
		Map:         res.MapName,
		PlayedAt:    e.OccurredAt,
		TotalRounds: totalRounds,
		Teams:       teams,
	}, nil
}

// kdRatio computes kills/deaths rounded to three decimals, treating zero deaths
// as one to avoid a divide-by-zero (mirrors the random generator's formula).
func kdRatio(kills, deaths int64) float64 {
	d := deaths
	if d == 0 {
		d = 1
	}
	return math.Round(float64(kills)/float64(d)*1000) / 1000
}

// teamSlot maps a contract teamKey ("team-1"/"team-2") to the match_teams
// team_slot column, which is constrained to 'A'/'B'.
func teamSlot(teamKey string) string {
	if teamKey == "team-2" {
		return "B"
	}
	return "A"
}

// matchResult classifies a team's outcome from its score versus the opponent's.
func matchResult(score, opponent int64) string {
	switch {
	case score > opponent:
		return "win"
	case score < opponent:
		return "loss"
	default:
		return "tie"
	}
}

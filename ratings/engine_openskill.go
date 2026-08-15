package ratings

import (
	"github.com/intinig/go-openskill/rating"
	"github.com/intinig/go-openskill/types"
)

// openSkillEngine rates players with the Weng-Lin (OpenSkill) system. It is
// rank-based: each match is a win / loss / draw decided by rounds won; margin of
// victory is not considered (a 13-2 and a 13-11 win are equivalent).
type openSkillEngine struct{}

// NewOpenSkillEngine returns the default rating engine.
func NewOpenSkillEngine() RatingEngine { return openSkillEngine{} }

func (openSkillEngine) Name() string { return "openskill" }

func (openSkillEngine) Compute(rosters []MatchRoster) []PlayerRating {
	// Each player carries an evolving rating across the season, starting fresh.
	current := make(map[int64]types.Rating)
	games := make(map[int64]int)
	get := func(id int64) types.Rating {
		r, ok := current[id]
		if !ok {
			r = rating.New()
			current[id] = r
		}
		return r
	}

	for _, m := range rosters {
		// Snapshot both teams' current ratings, in the stored player order.
		teams := make([]types.Team, len(m.Teams))
		for ti, tr := range m.Teams {
			team := make(types.Team, len(tr.PlayerIDs))
			for pi, pid := range tr.PlayerIDs {
				team[pi] = get(pid)
			}
			teams[ti] = team
		}

		// OpenSkill is rank-based: the score only decides win / loss / draw, not
		// the margin. Passing rounds won lets it pick the winner (more rounds) or a
		// draw (equal rounds).
		updated := rating.Rate(teams, &types.OpenSkillOptions{
			Score: []int{m.Teams[0].RoundsWon, m.Teams[1].RoundsWon},
		})

		for ti, tr := range m.Teams {
			for pi, pid := range tr.PlayerIDs {
				current[pid] = updated[ti][pi]
				games[pid]++
			}
		}
	}

	out := make([]PlayerRating, 0, len(current))
	for pid, r := range current {
		out = append(out, PlayerRating{
			PlayerID:    pid,
			Mu:          r.Mu,
			Sigma:       r.Sigma,
			Ordinal:     rating.Ordinal(r),
			GamesPlayed: games[pid],
		})
	}
	return out
}

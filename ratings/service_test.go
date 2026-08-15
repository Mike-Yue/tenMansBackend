package ratings

import (
	"context"
	"testing"
)

// fakeRepo feeds fixed rosters into the service and captures the rows it writes.
type fakeRepo struct {
	rosters []MatchRoster
	written []ratingRow
}

func (f *fakeRepo) SeasonMatchRosters(context.Context, int64) ([]MatchRoster, error) {
	return f.rosters, nil
}
func (f *fakeRepo) ReplaceSeasonRatings(_ context.Context, _ int64, rows []ratingRow) error {
	f.written = rows
	return nil
}
func (f *fakeRepo) ListSeasonIDs(context.Context) ([]int64, error) { return []int64{1}, nil }
func (f *fakeRepo) GetByPlayerSteamID(context.Context, int64) ([]seasonRatingRaw, bool, error) {
	return nil, false, nil
}

// oneMatch builds a single-match season: players 1-5 vs 6-10 with the given rounds.
func oneMatch(aRounds, bRounds int) []MatchRoster {
	return []MatchRoster{{
		MatchID: 1,
		Teams: [2]TeamRoster{
			{PlayerIDs: []int64{1, 2, 3, 4, 5}, RoundsWon: aRounds},
			{PlayerIDs: []int64{6, 7, 8, 9, 10}, RoundsWon: bRounds},
		},
	}}
}

// recompute runs the service over the rosters and returns player -> ordinal.
func recompute(t *testing.T, rosters []MatchRoster) map[int64]float64 {
	t.Helper()
	repo := &fakeRepo{rosters: rosters}
	svc := NewRatingService(repo, NewOpenSkillEngine())
	if err := svc.RecomputeSeason(context.Background(), 1); err != nil {
		t.Fatalf("RecomputeSeason: %v", err)
	}
	out := make(map[int64]float64, len(repo.written))
	for _, r := range repo.written {
		out[r.PlayerID] = r.Ordinal
	}
	return out
}

func TestWinnersOutrateLosers(t *testing.T) {
	ord := recompute(t, oneMatch(13, 3))
	if ord[1] <= ord[6] {
		t.Fatalf("winner (%.3f) should out-rate loser (%.3f)", ord[1], ord[6])
	}
}

// TestMarginIgnored documents the chosen behavior: OpenSkill is rank-based, so the
// round margin does not affect the rating — a blowout and a narrow win are equal.
func TestMarginIgnored(t *testing.T) {
	blowout := recompute(t, oneMatch(13, 3))
	narrow := recompute(t, oneMatch(13, 11))
	if blowout[1] != narrow[1] {
		t.Fatalf("expected margin to be ignored, got blowout %.6f vs narrow %.6f", blowout[1], narrow[1])
	}
}

func TestDeterministic(t *testing.T) {
	a := recompute(t, oneMatch(13, 7))
	b := recompute(t, oneMatch(13, 7))
	for id, v := range a {
		if b[id] != v {
			t.Fatalf("non-deterministic for player %d: %.6f vs %.6f", id, v, b[id])
		}
	}
}

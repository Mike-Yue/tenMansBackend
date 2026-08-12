package matches

import (
	"fmt"
	"math"
	"math/rand/v2"
	"time"
)

// This file is a temporary stand-in for the real match-creation flow (upload a
// .dem to S3, then have a parser service return real match/stats data). Until
// that exists, a POST simply fabricates a plausible random match.

const (
	playersPerTeam = 5
	winningRounds  = 13 // CS2 MR12: first to 13 wins
)

var mapPool = []string{"dust2", "mirage", "inferno"}

// generateRandomMatch builds a NewMatch by shuffling the given players into two
// teams of five and inventing a scoreline and per-player stats. Season is fixed
// to 1 for now. The caller must pass at least playersPerTeam*2 player IDs.
func generateRandomMatch(playerIDs []int64) NewMatch {
	shuffled := append([]int64(nil), playerIDs...)
	rand.Shuffle(len(shuffled), func(i, j int) {
		shuffled[i], shuffled[j] = shuffled[j], shuffled[i]
	})
	teamA := shuffled[:playersPerTeam]
	teamB := shuffled[playersPerTeam : playersPerTeam*2]

	// Winner takes 13; loser gets 0..12. No ties.
	loserRounds := int64(rand.IntN(winningRounds))
	aRounds, bRounds := int64(winningRounds), loserRounds
	aResult, bResult := "win", "loss"
	if rand.IntN(2) == 0 {
		aRounds, bRounds = loserRounds, winningRounds
		aResult, bResult = "loss", "win"
	}

	// One team starts CT, the other T.
	aSide, bSide := "CT", "T"
	if rand.IntN(2) == 0 {
		aSide, bSide = "T", "CT"
	}

	now := time.Now().Format("2006-01-02")
	totalRounds := aRounds + bRounds

	return NewMatch{
		Map:         mapPool[rand.IntN(len(mapPool))],
		PlayedAt:    now,
		UploadHash:  randomHash(),
		StorageKey:  "random/" + randomHash() + ".dem",
		SeasonID:    1,
		TotalRounds: totalRounds,
		Teams: []NewTeam{
			{TeamSlot: "A", StartingSide: aSide, RoundsWon: aRounds, Result: aResult, Players: randomPlayerStats(teamA, totalRounds)},
			{TeamSlot: "B", StartingSide: bSide, RoundsWon: bRounds, Result: bResult, Players: randomPlayerStats(teamB, totalRounds)},
		},
	}
}

func randomPlayerStats(playerIDs []int64, totalRounds int64) []NewPlayerStat {
	stats := make([]NewPlayerStat, 0, len(playerIDs))
	for _, id := range playerIDs {
		kills := int64(rand.IntN(26) + 5)  // 5..30
		deaths := int64(rand.IntN(21) + 5) // 5..25 (never 0, so KD is safe)
		assists := int64(rand.IntN(13))    // 0..12
		mvps := int64(rand.IntN(6))        // 0..5
		kd := math.Round(float64(kills)/float64(deaths)*1000) / 1000
		stats = append(stats, NewPlayerStat{
			PlayerID:      id,
			Kills:         kills,
			Deaths:        deaths,
			Assists:       assists,
			KDRatio:       kd,
			MVPs:          mvps,
			HeadshotKills: int64(rand.IntN(int(kills) + 1)),  // 0..kills
			TotalDamage:   kills*100 + int64(rand.IntN(400)), // ~100 ADR-ish per kill + noise
			UtilityDamage: int64(rand.IntN(301)),             // 0..300
			DamageAssists: int64(rand.IntN(9)),               // 0..8
			FlashAssists:  int64(rand.IntN(6)),               // 0..5
			RoundsPlayed:  totalRounds,
		})
	}
	return stats
}

// randomHash returns a 32-char hex string standing in for a real demo upload hash.
func randomHash() string {
	return fmt.Sprintf("%016x%016x", rand.Uint64(), rand.Uint64())
}

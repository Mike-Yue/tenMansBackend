package main

import (
	"log"
	"net/http"
	"os"

	"tenMansBackend/db"
	"tenMansBackend/matches"
	"tenMansBackend/ratings"
	"tenMansBackend/seasons"
	"tenMansBackend/stats"
	"tenMansBackend/users"
)

func main() {
	// DB_PATH points at the database file. On Render this is the persistent
	// disk (e.g. /data/production.db); locally it defaults to production.db.
	dbPath := os.Getenv("DB_PATH")
	if dbPath == "" {
		dbPath = "production.db"
	}

	database, err := db.Open(dbPath)
	if err != nil {
		log.Fatal(err)
	}
	defer database.Close()

	// Apply any pending schema migrations before serving traffic. On a fresh disk this
	// builds the schema from scratch; on an existing database it applies only what's new.
	if err := db.Migrate(database); err != nil {
		log.Fatal(err)
	}
	log.Printf("Connected to %s", dbPath)

	mux := http.NewServeMux()

	// Health check (used by Render); intentionally does not touch the DB.
	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	userRepo := users.NewUserRepository(database)
	userSvc := users.NewUserService(userRepo)
	userHandler := users.NewUserHandler(userSvc)
	userHandler.RegisterRoutes(mux)

	seasonRepo := seasons.NewSeasonRepository(database)
	seasonSvc := seasons.NewSeasonService(seasonRepo)
	seasonHandler := seasons.NewSeasonHandler(seasonSvc)
	seasonHandler.RegisterRoutes(mux)

	// Ratings are constructed before matches so the match service can call back
	// into them to recompute a season after a match is added or removed.
	ratingRepo := ratings.NewRatingRepository(database)
	ratingSvc := ratings.NewRatingService(ratingRepo, ratings.NewOpenSkillEngine())
	ratingHandler := ratings.NewRatingHandler(ratingSvc)
	ratingHandler.RegisterRoutes(mux)

	matchRepo := matches.NewMatchRepository(database)
	matchSvc := matches.NewMatchService(matchRepo, matches.NewStubPresigner(), ratingSvc)
	matchHandler := matches.NewMatchHandler(matchSvc)
	matchHandler.RegisterRoutes(mux)

	statsRepo := stats.NewStatsRepository(database)
	statsSvc := stats.NewStatsService(statsRepo)
	statsHandler := stats.NewStatsHandler(statsSvc)
	statsHandler.RegisterRoutes(mux)

	// Render provides the port to listen on via $PORT.
	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	server := &http.Server{
		Addr:    ":" + port,
		Handler: corsMiddleware(mux),
	}

	log.Printf("Server listening on :%s", port)
	if err := server.ListenAndServe(); err != nil {
		log.Fatal(err)
	}
}

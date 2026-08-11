package main

import (
	"log"
	"net/http"
	"os"

	"tenMansBackend/db"
	"tenMansBackend/matches"
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

	// if err := db.EnsureSeeded(dbPath); err != nil {
	// 	log.Fatal(err)
	// }

	database, err := db.Open(dbPath)
	if err != nil {
		log.Fatal(err)
	}
	defer database.Close()
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

	matchRepo := matches.NewMatchRepository(database)
	matchSvc := matches.NewMatchService(matchRepo, matches.NewStubPresigner())
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

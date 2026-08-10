package main

import (
	"log"
	"net/http"

	"tenMansBackend/db"
	"tenMansBackend/matches"
	"tenMansBackend/stats"
	"tenMansBackend/users"
)

func main() {
	database, err := db.Open("production.db")
	if err != nil {
		log.Fatal(err)
	}
	defer database.Close()
	log.Println("Connected to production.db")

	mux := http.NewServeMux()

	userRepo := users.NewUserRepository(database)
	userSvc := users.NewUserService(userRepo)
	userHandler := users.NewUserHandler(userSvc)
	userHandler.RegisterRoutes(mux)

	matchRepo := matches.NewMatchRepository(database)
	matchSvc := matches.NewMatchService(matchRepo)
	matchHandler := matches.NewMatchHandler(matchSvc)
	matchHandler.RegisterRoutes(mux)

	statsRepo := stats.NewStatsRepository(database)
	statsSvc := stats.NewStatsService(statsRepo)
	statsHandler := stats.NewStatsHandler(statsSvc)
	statsHandler.RegisterRoutes(mux)

	server := &http.Server{
		Addr:    ":8080",
		Handler: mux,
	}

	log.Println("Server listening on http://localhost:8080")
	if err := server.ListenAndServe(); err != nil {
		log.Fatal(err)
	}
}

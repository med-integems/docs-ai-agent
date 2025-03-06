package main

import (
	"fmt"
	"log"
	"net/http"

	"github.com/integems/report-agent/config"
	"github.com/integems/report-agent/src/database"
	"github.com/integems/report-agent/src/handlers"
)

var PORT = config.GetEnv("PORT", "4000")

// CORS middleware
func corsMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Set CORS headers
		w.Header().Set("Access-Control-Allow-Origin", "*") // Allow all origins
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")

		// Handle preflight requests
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}

		next.ServeHTTP(w, r)
	})
}

func main() {
	// Connect to a database
	db := database.NewDatabaseConnection()

	// Migrate tables
	if err := database.AutoMigrateTables(db); err != nil {
		log.Fatalf("Failed to migrate table: %v", err)
	}

	// Mux server
	mux := http.NewServeMux()

	// Initialize handlers
	handler := handlers.NewHandler(mux, db)
	handler.RegisterHandlers()

	// Wrap mux with CORS middleware
	corsEnabledHandler := corsMiddleware(mux)

	// Server
	server := http.Server{
		Addr:    fmt.Sprintf(":%v", PORT),
		Handler: corsEnabledHandler,
	}

	log.Printf("Server listening on port %v", PORT)
	log.Fatal(server.ListenAndServe())
}

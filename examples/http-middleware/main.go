package main

import (
	"fmt"
	"net/http"

	"github.com/jozefvalachovic/logger/v3"
	"github.com/jozefvalachovic/logger/v3/middleware"
)

func main() {
	// Create HTTP router
	mux := http.NewServeMux()

	// Add routes
	mux.HandleFunc("/hello", func(w http.ResponseWriter, r *http.Request) {
		logger.LogInfo("Processing hello request", "path", r.URL.Path)
		_, _ = fmt.Fprintln(w, "Hello, World!")
	})

	mux.HandleFunc("/error", func(w http.ResponseWriter, r *http.Request) {
		logger.LogError("Simulating error", "path", r.URL.Path)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
	})

	// Apply logging middleware
	// Second parameter (true) enables logging request/response body on errors
	loggedMux := middleware.LogHTTPMiddleware(mux, true)

	// Start server
	logger.LogInfo("Starting HTTP server", "port", 8080)
	if err := http.ListenAndServe(":8080", loggedMux); err != nil {
		logger.LogError("Server failed", "error", err.Error())
	}
}
